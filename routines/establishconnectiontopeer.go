package routines

import (
	"encoding/json"
	"harmony/backend/model"
)

type ECTPState int

const (
	ectp_entry ECTPState = iota
	ectp_bAcceptOrReject
	ectp_aSdpAnswer
	ectp_iceCandidates
)

type EstablishConnectionToPeer struct {
	pkA   *model.PublicKey
	pkB   *model.PublicKey
	hub   *model.Hub
	state ECTPState
}

func newEstablishConnectionToPeer(client *model.Client, hub *model.Hub) model.Routine {
	return &EstablishConnectionToPeer{
		hub:   hub,
		state: ectp_entry,
	}
}

func (r *EstablishConnectionToPeer) Next(args model.RoutineInput) []model.RoutineOutput {

	switch r.state {
	case ectp_entry:
		return r.entry(args)
	case ectp_bAcceptOrReject:
		return r.bAcceptOrReject(args)
	case ectp_aSdpAnswer:
		return r.aSdpAnswer(args)
	case ectp_iceCandidates:
		return r.iceCandidates(args)
	default:
		panic("unrecognized state?")
	}

}

func (r *EstablishConnectionToPeer) entry(args model.RoutineInput) []model.RoutineOutput {

	// store public key of first peer
	r.pkA = args.Pk

	// parse first message
	usrMsg := struct {
		Initiate string `json:"initiate"`
		Key      string `json:"key"`
	}{}
	json.Unmarshal([]byte(args.Msg), &usrMsg)
	r.pkB, _ = parsePublicKey(usrMsg.Key)

	_, peerOnline := r.hub.GetClient(*r.pkB)

	if peerOnline {
		r.state = ectp_bAcceptOrReject
		return []model.RoutineOutput{
			{
				Pk: r.pkB,
				Msgs: []string{`{
					"initiate": "receiveConnectionRequest",
					"key": "` + publicKeyToString(*r.pkA) + `"
				}`},
			},
		}
	} else {
		return []model.RoutineOutput{
			{
				Pk: r.pkA,
				Msgs: []string{`{
					"peerStatus": "offline",
					"forwarded": null,
					"terminate": "done"
				}`},
				Done: true,
			},
		}
	}

}

func (r *EstablishConnectionToPeer) bAcceptOrReject(args model.RoutineInput) []model.RoutineOutput {

	usrMsg := struct {
		Forward struct {
			Type string `json:"type"`
		} `json:"forward"`
	}{}
	json.Unmarshal([]byte(args.Msg), &usrMsg)

	switch usrMsg.Forward.Type {
	case "reject":
		return []model.RoutineOutput{
			{
				Pk: r.pkA,
				Msgs: []string{`{
					"peerStatus": "online",
					"forwarded": {
						"type": "reject"
					},
					"terminate": "done"
				}`},
				Done: true,
			},
			{
				Pk: r.pkB,
				Msgs: []string{`{
					"terminate": "done"
				}`},
				Done: true,
			},
		}

	case "acceptAndOffer":

		// unmarshal the "payload" bit of the usrmsg
		usrMsgWithPayload := struct {
			Forward struct {
				Payload struct {
					Type string `json:"type"`
					Sdp  string `json:"sdp"`
				} `json:"payload"`
			} `json:"forward"`
		}{}
		json.Unmarshal([]byte(args.Msg), &usrMsgWithPayload)

		// create message to B
		// marshal it instead of creating the json string directly so that the SDPs get sanitized
		dataToB := struct {
			PeerStatus string `json:"peerStatus"`
			Forwarded  struct {
				Type    string `json:"type"`
				Payload struct {
					Type string `json:"type"`
					Sdp  string `json:"sdp"`
				} `json:"payload"`
			} `json:"forwarded"`
		}{}
		dataToB.PeerStatus = "online"
		dataToB.Forwarded.Type = "acceptAndOffer"
		dataToB.Forwarded.Payload.Type = "offer"
		dataToB.Forwarded.Payload.Sdp = usrMsgWithPayload.Forward.Payload.Sdp

		msgToA, _ := json.Marshal(dataToB)

		r.state = ectp_aSdpAnswer
		return []model.RoutineOutput{
			{
				Pk:   r.pkA,
				Msgs: []string{string(msgToA)},
			},
		}

	default:
		panic(args.Msg)
	}

}

func (r *EstablishConnectionToPeer) aSdpAnswer(args model.RoutineInput) []model.RoutineOutput {

	// parse msg
	usrMsg := struct {
		Forward struct {
			Type    string `json:"type"`
			Payload struct {
				Type string `json:"type"`
				Sdp  string `json:"sdp"`
			} `json:"payload"`
		} `json:"forward"`
	}{}
	json.Unmarshal([]byte(args.Msg), &usrMsg)

	// remarshal it for B
	dataToB := struct {
		Forwarded struct {
			Type    string `json:"type"`
			Payload struct {
				Type string `json:"type"`
				Sdp  string `json:"sdp"`
			} `json:"payload"`
		} `json:"forwarded"`
	}{}
	dataToB.Forwarded.Type = "answer"
	dataToB.Forwarded.Payload.Type = "answer"
	dataToB.Forwarded.Payload.Sdp = usrMsg.Forward.Payload.Sdp
	msgToB, _ := json.Marshal(dataToB)

	r.state = ectp_iceCandidates
	return []model.RoutineOutput{
		{
			Pk:   r.pkB,
			Msgs: []string{string(msgToB)},
		},
	}
}

func (r *EstablishConnectionToPeer) iceCandidates(args model.RoutineInput) []model.RoutineOutput {
	return []model.RoutineOutput{}
}
