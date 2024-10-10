package routines

import "harmony/backend/model"

type EstablishConnectionToPeer struct {
}

func newEstablishConnectionToPeer(client *model.Client, hub *model.Hub) model.Routine {
	return &EstablishConnectionToPeer{}
}

func (r *EstablishConnectionToPeer) Next(args model.RoutineInput) []model.RoutineOutput {
	return []model.RoutineOutput{model.MakeRoutineOutput(true)}
}
