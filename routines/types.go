package routines

import "harmony/backend/model"

type RoutineConstructor func(*model.Client, *model.Hub) model.Routine

type RoutineConstructors struct {
	NewComeOnline                RoutineConstructor
	NewEstablishConnectionToPeer RoutineConstructor
	NewFriendRequest             RoutineConstructor
}
