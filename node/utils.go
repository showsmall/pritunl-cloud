package node

import (
	"github.com/pritunl/mongo-go-driver/bson"
	"github.com/pritunl/mongo-go-driver/bson/primitive"
	"github.com/pritunl/mongo-go-driver/mongo/options"
	"github.com/pritunl/pritunl-cloud/database"
	"github.com/pritunl/pritunl-cloud/utils"
)

func Get(db *database.Database, nodeId primitive.ObjectID) (
	nde *Node, err error) {

	coll := db.Nodes()
	nde = &Node{}

	err = coll.FindOneId(nodeId, nde)
	if err != nil {
		return
	}

	return
}

func GetAll(db *database.Database) (nodes []*Node, err error) {
	coll := db.Nodes()
	nodes = []*Node{}

	cursor, err := coll.Find(db, bson.M{})
	if err != nil {
		err = database.ParseError(err)
		return
	}
	defer cursor.Close(db)

	for cursor.Next(db) {
		nde := &Node{}
		err = cursor.Decode(nde)
		if err != nil {
			err = database.ParseError(err)
			return
		}

		nde.SetActive()
		nodes = append(nodes, nde)
	}

	err = cursor.Err()
	if err != nil {
		err = database.ParseError(err)
		return
	}

	return
}

func GetAllHypervisors(db *database.Database, query *bson.M) (
	nodes []*Node, err error) {

	coll := db.Nodes()
	nodes = []*Node{}

	cursor, err := coll.Find(
		db,
		query,
		&options.FindOptions{
			Sort: &bson.D{
				{"name", 1},
			},
			Projection: &bson.D{
				{"name", 1},
				{"types", 1},
				{"gui", 1},
				{"available_vpcs", 1},
				{"oracle_subnets", 1},
			},
		},
	)
	if err != nil {
		err = database.ParseError(err)
		return
	}
	defer cursor.Close(db)

	for cursor.Next(db) {
		nde := &Node{}
		err = cursor.Decode(nde)
		if err != nil {
			err = database.ParseError(err)
			return
		}

		if !nde.IsHypervisor() {
			continue
		}
		nde.JsonHypervisor()

		nodes = append(nodes, nde)
	}

	err = cursor.Err()
	if err != nil {
		err = database.ParseError(err)
		return
	}

	return
}

func GetAllPaged(db *database.Database, query *bson.M,
	page, pageCount int64) (nodes []*Node, count int64, err error) {

	coll := db.Nodes()
	nodes = []*Node{}

	count, err = coll.CountDocuments(db, query)
	if err != nil {
		err = database.ParseError(err)
		return
	}

	maxPage := count / pageCount
	if count == pageCount {
		maxPage = 0
	}
	page = utils.Min64(page, maxPage)
	skip := utils.Min64(page*pageCount, count)

	cursor, err := coll.Find(
		db,
		query,
		&options.FindOptions{
			Sort: &bson.D{
				{"name", 1},
			},
			Skip:  &skip,
			Limit: &pageCount,
		},
	)
	if err != nil {
		err = database.ParseError(err)
		return
	}
	defer cursor.Close(db)

	for cursor.Next(db) {
		nde := &Node{}
		err = cursor.Decode(nde)
		if err != nil {
			err = database.ParseError(err)
			return
		}

		nde.SetActive()
		nodes = append(nodes, nde)
	}

	err = cursor.Err()
	if err != nil {
		err = database.ParseError(err)
		return
	}

	return
}

func GetAllNet(db *database.Database) (nodes []*Node, err error) {
	coll := db.Nodes()
	nodes = []*Node{}

	opts := &options.FindOptions{
		Projection: &bson.D{
			{"zone", 1},
			{"private_ips", 1},
		},
	}

	cursor, err := coll.Find(db, bson.M{}, opts)
	if err != nil {
		err = database.ParseError(err)
		return
	}
	defer cursor.Close(db)

	for cursor.Next(db) {
		nde := &Node{}
		err = cursor.Decode(nde)
		if err != nil {
			err = database.ParseError(err)
			return
		}

		nodes = append(nodes, nde)
	}

	err = cursor.Err()
	if err != nil {
		err = database.ParseError(err)
		return
	}

	return
}

func Remove(db *database.Database, nodeId primitive.ObjectID) (err error) {
	coll := db.Nodes()

	_, err = coll.DeleteOne(db, &bson.M{
		"_id": nodeId,
	})
	if err != nil {
		err = database.ParseError(err)
		switch err.(type) {
		case *database.NotFoundError:
			err = nil
		default:
			return
		}
	}

	return
}
