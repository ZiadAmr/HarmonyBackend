package routines

import "harmony/backend/model"

type EstablishConnectionToPeer struct {
}

func newEstablishConnectionToPeer(client *model.Client, hub *model.Hub) model.Routine {
	return &EstablishConnectionToPeer{}
}

func (r *EstablishConnectionToPeer) Next(msg string) model.RoutineOutput {
	return model.MakeRoutineOutput(true)
}
