# --arg symbol 'full/package.(Receiver).Method'; include every verdict/suppression.
[.cases[] | select(.symbol == $symbol)]
