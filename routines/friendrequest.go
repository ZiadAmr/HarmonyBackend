package routines

import "harmony/backend/model"

type FriendRequest struct {
}

func newFriendRequest(client *model.Client, hub *model.Hub) model.Routine {
	return &FriendRequest{}
}

func (f *FriendRequest) Next(args model.RoutineInput) []model.RoutineOutput {
	return []model.RoutineOutput{}
}
