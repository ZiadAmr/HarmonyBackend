package routines

import "harmony/backend/model"

// abstracted routine functions for testing/dependency injection
type Routines interface {
	ComeOnline(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string)
	EstablishConnectionToPeer(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string)
}

// Concrete routine definitions. Implements Routines. Methods defined elsewhere
type RoutinesDefn struct{}

type RoutineConstructor func(*model.Client, *model.Hub) model.Routine

type RoutineConstructors struct {
	NewComeOnline                RoutineConstructor
	NewEstablishConnectionToPeer RoutineConstructor
}
