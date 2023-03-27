package main

func clamp(x, min, max int) int {
	switch {
	case x < min:
		return min
	case x > max:
		return max
	default:
		return x
	}
}

func clampf(x, min, max float32) float32 {
	switch {
	case x < min:
		return min
	case x > max:
		return max
	default:
		return x
	}
}
