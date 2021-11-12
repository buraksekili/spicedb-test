package main

import (
	"context"
	"fmt"
	"log"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
)

const Schema = `definition user {
	relation administration: administration
	relation user: user

	// By default, all users can create a user.
	permission create = user + administration->member
}

definition administration {
	relation member: user
}

definition thing {
	relation administration: administration#member
	// relation group: group

	// Thing entity roles
	relation owner: user | administration#member
	relation reader: user 
	relation writer: user 
	relation deleter: user 
	// relation accesser: user 
	relation accesser: user | group#user_member


	// Thing entity Actions
	permission read = owner + administration + reader + accesser
	permission write = owner + administration + writer + accesser
	permission update = owner + administration + deleter + accesser
	permission access = read + writer + deleter
}
definition group {
	relation administration: administration | administration#member
	relation owner: user
	relation user_member: user | group#user_member
	relation thing_member: thing
	relation group_access: group

	// permissions
	permission assign = owner + administration->member
	permission unassign = owner + administration->member
	permission access = owner + user_member + administration->member + group_access->user_member
}`

func applySchema(client *authzed.Client) error {
	request := &pb.WriteSchemaRequest{Schema: Schema}
	if _, err := client.WriteSchema(context.Background(), request); err != nil {
		return fmt.Errorf("failed to write schema: %s", err)
	}
	log.Println("schema applied succesfully")
	return nil
}
