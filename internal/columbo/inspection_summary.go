package columbo

import "context"

func (r *snapshotRenderer) sourceGuidance() error {
	available, err := r.queries.InspectionAvailable(context.Background())
	if err != nil || !available {
		return err
	}
	r.emit("Source evidence: source_receiver_seeds, source_receiver_siblings, source_storage_paths and source_one_hop_calls navigate unsuppressed ordinary non-prose method-linked cases. For stage-only findings, use advisory_receipts with source_declarations, source_calls and source_fields; source_ranges retain exact locations.\n")
	r.emit("Before editing: verify source_files.sha256 and source_ranges.sha256 against current bytes. Reproduce scope with source_scan.root, the PWD entry in source_environment and source_patterns; rerun after changes.\n")
	r.emit("Evidence limits: receiver leads and selected-runtime siblings, declared struct storage paths and one-hop calls do not prove aliases, ownership, lifetime or effects. Inspect the source before deciding.\n")
	return nil
}
