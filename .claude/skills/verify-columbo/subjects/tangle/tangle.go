package tangle

type Item struct {
	Name           string
	Quality, SellIn int
}

func update(x *Item) {
	if x.Name != "A" && !(x.Name == "B") {
		x.Quality--
		if x.Name != "B" {
			x.SellIn--
		}
	}
	if x.Name == "A" {
		x.Quality++
	}
}
