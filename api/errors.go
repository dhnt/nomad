package api

type ErrorNotFound struct {
	Status string
}

func (e ErrorNotFound) Error() string {
	return e.Status
}
