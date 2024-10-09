package routines

import "harmony/backend/model"

type EstablishConnectionToPeer struct {
}

func newEstablishConnectionToPeer(client *model.Client, hub *model.Hub) model.Routine {
	return &EstablishConnectionToPeer{}
}

func (r *EstablishConnectionToPeer) Next(msgType model.RoutineMsgType, pk *model.PublicKey, msg string) []model.RoutineOutput {
	return []model.RoutineOutput{model.MakeRoutineOutput(true)}
}
