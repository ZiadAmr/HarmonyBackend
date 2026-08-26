package routines

import (
	"harmony/backend/model"
	"log/slog"
)

type RoutineConstructor func(*model.Client, *model.Hub, *slog.Logger) model.Routine

type RoutineConstructors struct {
	NewComeOnline                RoutineConstructor
	NewEstablishConnectionToPeer RoutineConstructor
	NewFriendRequest             RoutineConstructor
	NewFriendRejection           RoutineConstructor
}
