package tui

// formMode identifies which inline form is currently active.
type formMode int

// Form modes displayed above the list.
const (
	formNone formMode = iota
	formAdd
	formEditTitle
	formTags
	formDue
	formSearch
	formSort
)
