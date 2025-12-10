package trust

type Tristate byte

func (t Tristate) Is(val bool) bool {
	if val {
		return t == TristateTrue
	} else {
		return t == TristateFalse
	}
}

const TristateDefault Tristate = 0
const TristateTrue Tristate = 1
const TristateFalse Tristate = 2
