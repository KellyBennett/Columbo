package report

import (
	"database/sql"
	"fmt"
	"io"
)

// scored keeps trials where the agent got to work; an infrastructure failure is retried, never scored.
const scored = `t AS (SELECT * FROM trials WHERE infra_error = '')`

// latestLeads reads the newest lead-check batch per corpus, as the canonical report reads the newest run per operator.
const latestLeads = `latest AS (SELECT corpus_module, max(batch) AS batch FROM lead_checks GROUP BY corpus_module),
l AS (SELECT c.* FROM lead_checks c JOIN latest USING (corpus_module, batch))`

var trialSections = []section{
	{
		title: "Setup: agent trials",
		note: "Each agent got a fresh copy of one killed inject mutant and Columbo's output for the cases the mutant added, and nothing about how the mutant was made. A different model is a flag change (`--model`), and its trials report separately. " +
			"`agent_read` is the Columbo build whose output the agent read and ran; `scored_by` planted the mutant and scored the fix. They differ only for a trial rescored from its stored fix, which `trials rescore` does only when `scored_by` gives the mutant the same prompt the agent saw.",
		query: `WITH ` + scored + `
SELECT model, model_id, prompt_version, count(*) AS trials, agent_columbo AS agent_read, columbo_version AS scored_by, validity, agent_command
FROM t GROUP BY model, model_id, prompt_version, agent_command, agent_columbo, columbo_version ORDER BY model, prompt_version, agent_columbo`,
	},
	{
		title: "Setup: lead checks",
		note:  "The Columbo build behind the newest lead-check batch for each corpus.",
		query: `WITH ` + latestLeads + `
SELECT columbo_version AS columbo, validity, count(DISTINCT corpus_module) AS corpora, count(DISTINCT corpus_module || ' ' || case_id) AS cases, count(*) AS leads
FROM l GROUP BY columbo_version, validity ORDER BY columbo_version, validity`,
	},
	{
		title: "Outcomes by smell",
		note: "Counts read k/n over the smell's n trials. Cleared: Columbo no longer reports the planted smell anywhere beyond the unmutated baseline, and the trial is not gamed. " +
			"Gamed: the fix added a suppression, or put code where Columbo does not look (behind a generated-code header, in an excluded or unselected file, or in a test file); a gamed trial never counts as cleared. " +
			"Tests held: the mutant's own test files, restored over the fix, pass, or they no longer compile because the fix changed a signature they call (adapted) while the agent's updated tests pass, keep every original test function, and call no function the fix removed. " +
			"Edited tests: the agent changed a test file. Changed out of scope: the fix removed or moved a case the prompt said to leave alone. " +
			"Green: cleared, vet passes, tests held, the mutant's other cases are gone, and no case appeared that neither the baseline nor the mutant had. " +
			"Fix distance is token edits from the fix to the original divided by token edits from the mutant to the original: 0 restored the original, 1 got no closer, above 1 moved further away. Quartiles use the nearest rank.",
		query: `WITH ` + scored + `,
q AS (SELECT model, prompt_version, columbo_version, smell, fix_distance,
 row_number() OVER (PARTITION BY model, prompt_version, columbo_version, smell ORDER BY fix_distance) AS rn,
 count(*) OVER (PARTITION BY model, prompt_version, columbo_version, smell) AS n FROM t),
d AS (SELECT model, prompt_version, columbo_version, smell, min(fix_distance) AS lo,
 max(CASE WHEN rn = (n + 3) / 4 THEN fix_distance END) AS p25,
 avg(CASE WHEN rn IN ((n + 1) / 2, (n + 2) / 2) THEN fix_distance END) AS med,
 max(CASE WHEN rn = (3 * n + 3) / 4 THEN fix_distance END) AS p75,
 max(fix_distance) AS hi
 FROM q GROUP BY model, prompt_version, columbo_version, smell)
SELECT t.smell, t.model, count(*) AS n, count(DISTINCT t.corpus_module || ' ' || t.site_symbol) AS sites,
 printf('%d/%d', sum(t.cleared), count(*)) AS cleared,
 printf('%d/%d', sum(t.gamed), count(*)) AS gamed,
 printf('%d/%d', sum(t.vet_pass = 1 AND t.tests IN ('pass', 'adapted')), count(*)) AS tests_held,
 printf('%d/%d', sum(t.tests = 'adapted'), count(*)) AS adapted,
 printf('%d/%d', sum(t.test_files_changed > 0), count(*)) AS edited_tests,
 printf('%d/%d', sum(t.introduced > 0), count(*)) AS introduced_new_cases,
 printf('%d/%d', sum(t.out_of_scope_gone > 0), count(*)) AS changed_out_of_scope,
 printf('%d/%d', sum(t.cleared AND t.vet_pass = 1 AND t.tests IN ('pass', 'adapted') AND t.introduced = 0 AND t.other_remaining = 0), count(*)) AS green,
 printf('%.2f', d.lo) AS distance_min, printf('%.2f', d.p25) AS p25, printf('%.2f', d.med) AS median, printf('%.2f', d.p75) AS p75, printf('%.2f', d.hi) AS distance_max
FROM t JOIN d ON d.model = t.model AND d.prompt_version = t.prompt_version AND d.columbo_version = t.columbo_version AND d.smell = t.smell
GROUP BY t.model, t.prompt_version, t.columbo_version, t.smell ORDER BY t.model, t.prompt_version, t.columbo_version, t.smell`,
	},
	{
		title: "Agent runs",
		note:  "Wall time is per agent run, summed; runs overlapped, so elapsed time was shorter. Infrastructure errors are runs where the agent never started work; a rerun retries them and they are not scored.",
		query: `SELECT model, count(*) AS runs, sum(infra_error <> '') AS infra_errors, sum(infra_error = '' AND timed_out) AS timeouts,
 sum(agent_stop = 'error_max_turns') AS turn_cap_stops, sum(infra_error = '' AND agent_exit <> 0) AS nonzero_exits,
 printf('%.1f', sum(wall_ms) / 60000.0) AS agent_minutes, printf('%.2f', sum(cost_usd)) AS cost_usd,
 min(turns) AS min_turns, max(turns) AS max_turns
FROM trials GROUP BY model ORDER BY model`,
	},
	{
		title: "Trials",
		note: "One row per scored trial. Tests is how the mutant's own test files fared against the fix (pass, fail, adapted, or broken); agent_tests is whether the tests as the agent left them pass. " +
			"Distances are raw token edits. Flags name what the fix did to Columbo's view rather than to the code: config (restored before scoring), suppression, generated-marker, excluded, and moved-to-test hide code, and removed-file only records that a file Columbo analyzed is gone.",
		query: `WITH ` + scored + `
SELECT smell, corpus_module AS corpus, site_symbol AS site, dose, repeat, agent_stop AS stop, turns,
 cleared, vet_pass AS vet, tests, agent_tests_pass AS agent_tests, introduced_cases, other_remaining, out_of_scope_gone_cases,
 distance_fixed, distance_mutant, printf('%.2f', fix_distance) AS fix_distance, files_changed, test_files_changed, flags,
 printf('%.0f', wall_ms / 1000.0) AS wall_s, printf('%.2f', cost_usd) AS cost_usd
FROM t ORDER BY smell, corpus_module, site_symbol, dose, repeat`,
	},
	{
		title: "Leads by smell",
		note: "Each lead Columbo wrote on a natural case in the corpora, and what applying the refactoring it names did to that same case, with no model involved. " +
			"Cleared: the cure removed the case and added nothing. Conflict: removed it but set off another rule. Survived: the case remains. " +
			"Not applicable: the cure has no mechanical application at that case. A lead that asks a question or says what to inspect names no refactoring.",
		query: `WITH ` + latestLeads + `
SELECT smell, lead,
 CASE WHEN refactoring = '' THEN 'names no refactoring' ELSE refactoring END AS refactoring,
 CASE WHEN refactoring <> '' AND operator = '' THEN 'no cure operator' ELSE operator END AS cure,
 count(*) AS cases, sum(outcome = 'cleared') AS cleared, sum(outcome = 'conflict') AS conflict, sum(outcome = 'survived') AS survived,
 sum(outcome = 'invalid') AS invalid, sum(outcome = 'not-applicable') AS not_applicable
FROM l GROUP BY smell, lead ORDER BY smell, min(lead_ordinal), lead`,
	},
	{
		title: "Leads by corpus",
		query: `WITH ` + latestLeads + `
SELECT corpus_module, corpus_version, validity, count(DISTINCT case_id) AS cases, count(*) AS leads,
 sum(operator <> '') AS leads_with_cure, sum(outcome = 'cleared') AS cleared, sum(outcome = 'conflict') AS conflict,
 sum(outcome = 'survived') AS survived, sum(outcome = 'invalid') AS invalid, sum(outcome = 'not-applicable') AS not_applicable
FROM l GROUP BY corpus_module ORDER BY corpus_module`,
	},
	{
		title: "Unclassified leads",
		note:  "Lead text the harness's lead table does not know. Each one needs a row in trials/leads.go before its check means anything.",
		query: `WITH ` + latestLeads + `
SELECT smell, lead, count(*) AS cases FROM l WHERE outcome = 'unclassified' GROUP BY smell, lead ORDER BY smell, lead`,
	},
}

// WriteTrials renders the phase 8 report from the trials and lead_checks tables alone.
func WriteTrials(w io.Writer, db *sql.DB) error {
	fmt.Fprintln(w, "# Agent trials and lead checks")
	for _, s := range trialSections {
		if _, err := writeSection(w, db, s); err != nil {
			return fmt.Errorf("%s: %w", s.title, err)
		}
	}
	return nil
}
