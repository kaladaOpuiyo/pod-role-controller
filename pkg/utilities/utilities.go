package utilities

func Find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

func Remove(slice []string, val string) []string {
	for i, item := range slice {
		if item == val {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
