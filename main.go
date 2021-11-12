package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/authzed/grpcutil"
	"github.com/authzed/spicedb/pkg/tuple"
	"google.golang.org/grpc"
)

type PolicyReq struct {
	Subject     string
	SubjectType string
	Object      string
	ObjectType  string
	Relation    string
}

func initializeClient() (*authzed.Client, error) {
	return authzed.NewClient(
		"localhost:50051",
		grpc.WithInsecure(),
		grpcutil.WithInsecureBearerToken("somerandomkeyhere"),
	)
}

func main() {
	client, err := initializeClient()
	if err != nil {
		log.Fatalf("unable to initialize client: %s", err)
	}
	if err := applySchema(client); err != nil {
		log.Fatal(err)
	}

	fmt.Println("\tadding policies")
	for i := 0; i < 10; i++ {
		err := add(client, PolicyReq{ObjectType: "thing", Object: fmt.Sprintf("thing-id-%d", i), Relation: "reader", SubjectType: "user", Subject: "user-123"})
		if err != nil {
			log.Println("failed to add policies: ", err.Error())
		}
	}
	time.Sleep(5 * time.Second)

	log.Println("\tchecking policies")
	for i := 0; i < 10; i++ {
		pr := PolicyReq{ObjectType: "thing", Object: fmt.Sprintf("thing-id-%d", i), Relation: "read", SubjectType: "user", Subject: "user-123"}
		err := check(client, pr)
		if err != nil {
			log.Printf("failed to check policies for %s: %v\n", pr.Object, err.Error())
		}
	}

	err = expand(client, PolicyReq{ObjectType: "thing", Relation: "read", Subject: "user-123", SubjectType: "user"})
	if err != nil {
		log.Println("failed to expand policies: ", err.Error())
	}

}

func add(client *authzed.Client, pr PolicyReq) error {
	req := &pb.WriteRelationshipsRequest{Updates: []*pb.RelationshipUpdate{{
		Operation:    pb.RelationshipUpdate_OPERATION_CREATE,
		Relationship: tuple.ParseRel(fmt.Sprintf("%s:%s#%s@%s:%s", pr.ObjectType, pr.Object, pr.Relation, pr.SubjectType, pr.Subject)),
	},
	}}
	_, err := client.WriteRelationships(context.Background(), req)
	if err != nil {
		return err
	}
	return nil
}

func check(client *authzed.Client, pr PolicyReq) error {
	subject := &pb.SubjectReference{Object: &pb.ObjectReference{
		ObjectType: pr.SubjectType,
		ObjectId:   pr.Subject,
	}}
	object := &pb.ObjectReference{ObjectType: pr.ObjectType, ObjectId: pr.Object}
	resp, err := client.CheckPermission(context.Background(), &pb.CheckPermissionRequest{
		Resource:   object,
		Permission: pr.Relation,
		Subject:    subject,
	})
	if err != nil {
		return err
	}
	if resp.GetPermissionship() != pb.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION {
		return fmt.Errorf("failed to obtain the permission")
	}
	return nil
}

func expand(client *authzed.Client, pr PolicyReq) error {
	objectNS := pr.ObjectType
	relation := pr.Relation
	subjectNS := pr.SubjectType
	subjectID, subjectRel := parseSubject(pr.Subject)

	request := &pb.LookupResourcesRequest{
		ResourceObjectType: objectNS,
		Permission:         relation,
		Subject: &pb.SubjectReference{
			Object: &pb.ObjectReference{
				ObjectType: subjectNS,
				ObjectId:   subjectID,
			},
			OptionalRelation: subjectRel,
		},
	}
	respStream, err := client.LookupResources(context.Background(), request)
	if err != nil {
		return fmt.Errorf("failed to create lookupresource stream")
	}

	counter := 0
	for {
		r, err := respStream.Recv()
		switch {
		case err == io.EOF:
			fmt.Println("DONE/EOF")
			return nil
		case err != nil:
			fmt.Println("DONE")
			return err
		default:
			i := r.ResourceObjectId
			if i == "" {
				fmt.Println("FINISHED")
				return nil
			}
			fmt.Printf("%d\t%s\n", counter, i)
		}
		counter++
	}
}

func parseSubject(subject string) (id, relation string) {
	sarr := strings.Split(subject, "#")
	if len(sarr) != 2 {
		return subject, ""
	}
	return sarr[0], sarr[1]
}
