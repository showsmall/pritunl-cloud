package vpc

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"github.com/dropbox/godropbox/container/set"
	"github.com/dropbox/godropbox/errors"
	"github.com/pritunl/mongo-go-driver/bson"
	"github.com/pritunl/mongo-go-driver/bson/primitive"
	"github.com/pritunl/mongo-go-driver/mongo/options"
	"github.com/pritunl/pritunl-cloud/database"
	"github.com/pritunl/pritunl-cloud/errortypes"
	"github.com/pritunl/pritunl-cloud/requires"
	"github.com/pritunl/pritunl-cloud/utils"
	"math/rand"
	"net"
	"strings"
)

type Route struct {
	Destination string `bson:"destination" json:"destination"`
	Target      string `bson:"target" json:"target"`
	Link        bool   `bson:"link" json:"link"`
}

type Vpc struct {
	Id           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name         string             `bson:"name" json:"name"`
	Comment      string             `bson:"comment" json:"comment"`
	VpcId        int                `bson:"vpc_id" json:"vpc_id"`
	Network      string             `bson:"network" json:"network"`
	Network6     string             `bson:"-" json:"network6"`
	Subnets      []*Subnet          `bson:"subnets" json:"subnets"`
	Organization primitive.ObjectID `bson:"organization" json:"organization"`
	Datacenter   primitive.ObjectID `bson:"datacenter" json:"datacenter"`
	Routes       []*Route           `bson:"routes" json:"routes"`
	curSubnets   []*Subnet          `bson:"-" json:"-"`
}

func (v *Vpc) Validate(db *database.Database) (
	errData *errortypes.ErrorData, err error) {

	if v.VpcId == 0 {
		errData = &errortypes.ErrorData{
			Error:   "vpc_id_invalid",
			Message: "Vpc ID invalid",
		}
		return
	}

	if v.Organization.IsZero() {
		errData = &errortypes.ErrorData{
			Error:   "organization_required",
			Message: "Missing required organization",
		}
		return
	}

	if v.Datacenter.IsZero() {
		errData = &errortypes.ErrorData{
			Error:   "datacenter_required",
			Message: "Missing required datacenter",
		}
		return
	}

	network, e := v.GetNetwork()
	if e != nil {
		errData = &errortypes.ErrorData{
			Error:   "network_invalid",
			Message: "Network address invalid",
		}
		return
	}

	network6, e := v.GetNetwork6()
	if e != nil {
		errData = &errortypes.ErrorData{
			Error:   "network_invalid6",
			Message: "IPv6 network address invalid",
		}
		return
	}

	v.Network = network.String()

	if v.Subnets == nil {
		v.Subnets = []*Subnet{}
	}

	subnetRanges := []struct {
		Id    primitive.ObjectID
		Start int64
		Stop  int64
	}{}
	subs := []*Subnet{}
	for _, sub := range v.Subnets {
		if sub.Network == "" {
			continue
		}

		subNetwork, e := sub.GetNetwork()
		if e != nil {
			errData = &errortypes.ErrorData{
				Error:   "subnet_network_invalid",
				Message: "Subnet network address invalid",
			}
			return
		}

		cidr, _ := subNetwork.Mask.Size()
		if cidr < 8 {
			errData = &errortypes.ErrorData{
				Error:   "subnet_network_size_invalid",
				Message: "Subnet network size too big",
			}
			return
		}
		if cidr > 28 {
			errData = &errortypes.ErrorData{
				Error:   "subnet_network_size_invalid",
				Message: "Subnet network size too small",
			}
			return
		}

		sub.Network = subNetwork.String()

		if !utils.NetworkContains(network, subNetwork) {
			errData = &errortypes.ErrorData{
				Error:   "subnet_network_range_invalid",
				Message: "Subnet network outside of VPC network",
			}
			return
		}

		subStart, subStop, e := sub.GetIndexRange()
		if e != nil {
			err = e
			return
		}

		subnetRanges = append(subnetRanges, struct {
			Id    primitive.ObjectID
			Start int64
			Stop  int64
		}{
			Id:    sub.Id,
			Start: subStart,
			Stop:  subStop,
		})

		subs = append(subs, sub)
	}
	v.Subnets = subs

	for _, sub := range v.Subnets {
		subStart, subStop, e := sub.GetIndexRange()
		if e != nil {
			err = e
			return
		}

		for _, s := range subnetRanges {
			if s.Id == sub.Id {
				continue
			}

			if (subStart >= s.Start && subStart <= s.Stop) ||
				(subStop >= s.Start && subStop <= s.Stop) {

				errData = &errortypes.ErrorData{
					Error:   "subnet_network_range_overlap",
					Message: "VPC cannot have overlapping subnets",
				}
				return
			}
		}
	}

	if v.Routes == nil {
		v.Routes = []*Route{}
	}

	destinations := set.NewSet()
	for _, route := range v.Routes {
		if destinations.Contains(route.Destination) {
			errData = &errortypes.ErrorData{
				Error:   "duplicate_destination",
				Message: "Duplicate route destinations",
			}
			return
		}
		destinations.Add(route.Destination)

		if strings.Contains(route.Destination, ":") !=
			strings.Contains(route.Target, ":") {

			errData = &errortypes.ErrorData{
				Error:   "route_target_destination_invalid",
				Message: "Route target/destination invalid",
			}
			return
		}

		_, destination, e := net.ParseCIDR(route.Destination)
		if e != nil {
			errData = &errortypes.ErrorData{
				Error:   "route_destination_invalid",
				Message: "Route destination invalid",
			}
			return
		}
		route.Destination = destination.String()

		if route.Destination == "0.0.0.0/0" || route.Destination == "::/0" {
			errData = &errortypes.ErrorData{
				Error:   "route_destination_invalid",
				Message: "Route destination invalid",
			}
			return
		}

		target := net.ParseIP(route.Target)
		if target == nil {
			errData = &errortypes.ErrorData{
				Error:   "route_target_invalid",
				Message: "Route target invalid",
			}
			return
		}
		route.Target = target.String()

		if route.Target == "0.0.0.0" {
			errData = &errortypes.ErrorData{
				Error:   "route_target_invalid",
				Message: "Route target invalid",
			}
			return
		}

		if !strings.Contains(route.Target, ":") {
			if !network.Contains(target) {
				errData = &errortypes.ErrorData{
					Error:   "route_target_invalid_network",
					Message: "Route target not in VPC network",
				}
				return
			}
		} else {
			if !network6.Contains(target) {
				errData = &errortypes.ErrorData{
					Error:   "route_target_invalid_network6",
					Message: "Route target not in VPC IPv6 network",
				}
				return
			}
		}
	}

	return
}

func (v *Vpc) PreCommit() {
	if v.Subnets == nil {
		v.curSubnets = []*Subnet{}
	} else {
		v.curSubnets = v.Subnets
	}
}

func (v *Vpc) PostCommit(db *database.Database) (
	errData *errortypes.ErrorData, err error) {

	curSubnets := map[primitive.ObjectID]*Subnet{}
	for _, sub := range v.curSubnets {
		curSubnets[sub.Id] = sub
	}

	newIds := set.NewSet()
	for _, sub := range v.Subnets {
		newIds.Add(sub.Id)

		curSub := curSubnets[sub.Id]
		if !sub.Id.IsZero() && curSub != nil {
			if curSub.Network != sub.Network {
				errData = &errortypes.ErrorData{
					Error:   "subnet_network_modified",
					Message: "Cannot modify VPC subnet",
				}
				return
			}
		} else {
			sub.Id = primitive.NewObjectID()

			for _, s := range v.curSubnets {
				if s.Network == sub.Network {
					sub.Id = s.Id
				}
			}
		}
	}

	for _, sub := range v.curSubnets {
		if !newIds.Contains(sub.Id) {
			err = v.RemoveSubnet(db, sub.Id)
			if err != nil {
				return
			}
		}
	}

	return
}

func (v *Vpc) Json() {
	netHash := md5.New()
	netHash.Write(v.Id[:])
	netHashSum := fmt.Sprintf("%x", netHash.Sum(nil))[:12]

	ip := fmt.Sprintf("fd97%s", netHashSum)
	ipBuf := bytes.Buffer{}

	for i, run := range ip {
		if i%4 == 0 && i != 0 && i != len(ip)-1 {
			ipBuf.WriteRune(':')
		}
		ipBuf.WriteRune(run)
	}

	v.Network6 = ipBuf.String() + "::/64"
}

func (v *Vpc) GetSubnet(id primitive.ObjectID) (sub *Subnet) {
	if v.Subnets == nil || id.IsZero() {
		return
	}

	for _, s := range v.Subnets {
		if s.Id == id {
			sub = s
			return
		}
	}

	return
}

func (v *Vpc) GetNetwork() (network *net.IPNet, err error) {
	_, network, err = net.ParseCIDR(v.Network)
	if err != nil {
		err = &errortypes.ParseError{
			errors.Wrap(err, "vpc: Failed to parse network"),
		}
		return
	}
	return
}

func (v *Vpc) GetNetwork6() (network *net.IPNet, err error) {
	netHash := md5.New()
	netHash.Write(v.Id[:])
	netHashSum := fmt.Sprintf("%x", netHash.Sum(nil))[:12]

	ip := fmt.Sprintf("fd97%s", netHashSum)
	ipBuf := bytes.Buffer{}

	for i, run := range ip {
		if i%4 == 0 && i != 0 && i != len(ip)-1 {
			ipBuf.WriteRune(':')
		}
		ipBuf.WriteRune(run)
	}

	_, network, err = net.ParseCIDR(ipBuf.String() + "::/64")
	if err != nil {
		err = &errortypes.ParseError{
			errors.Wrap(err, "vpc: Failed to parse network"),
		}
		return
	}

	return
}

func (v *Vpc) InitVpc() {
	v.VpcId = rand.Intn(4085) + 10

	if v.Subnets != nil {
		for _, sub := range v.Subnets {
			sub.Id = primitive.NewObjectID()
		}
	}
}

func (v *Vpc) GetGateway() (ip net.IP, err error) {
	network, err := v.GetNetwork()
	if err != nil {
		return
	}

	ip = network.IP
	utils.IncIpAddress(ip)

	return
}

func (v *Vpc) GetGateway6() (ip net.IP, err error) {
	network, err := v.GetNetwork6()
	if err != nil {
		return
	}

	ip = network.IP
	utils.IncIpAddress(ip)

	return
}

func (v *Vpc) GetIp(db *database.Database,
	subId, instId primitive.ObjectID) (instIp, gateIp net.IP, err error) {

	subnet := v.GetSubnet(subId)
	if subnet == nil {
		err = &errortypes.ReadError{
			errors.New("vpc: Subnet does not exist"),
		}
		return
	}

	coll := db.VpcsIp()
	vpcIp := &VpcIp{}

	err = coll.FindOne(db, &bson.M{
		"vpc":      v.Id,
		"instance": instId,
	}).Decode(vpcIp)
	if err != nil {
		err = database.ParseError(err)
		vpcIp = nil
		if _, ok := err.(*database.NotFoundError); ok {
			err = nil
		} else {
			return
		}
	}

	if vpcIp == nil {
		vpcIp = &VpcIp{}
		opts := &options.FindOneAndUpdateOptions{}
		opts.SetReturnDocument(options.After)

		err = coll.FindOneAndUpdate(
			db,
			&bson.M{
				"vpc":      v.Id,
				"subnet":   subId,
				"instance": nil,
			},
			&bson.M{
				"$set": &bson.M{
					"instance": instId,
				},
			},
			opts,
		).Decode(vpcIp)
		if err != nil {
			err = database.ParseError(err)
			vpcIp = nil
			if _, ok := err.(*database.NotFoundError); ok {
				err = nil
			} else {
				return
			}
		}
	}

	if vpcIp == nil {
		vpcIp = &VpcIp{}

		err = coll.FindOne(
			db,
			&bson.M{
				"vpc":    v.Id,
				"subnet": subId,
			},
			&options.FindOneOptions{
				Sort: &bson.D{
					{"ip", -1},
				},
			},
		).Decode(vpcIp)
		if err != nil {
			vpcIp = nil
			err = database.ParseError(err)
			if _, ok := err.(*database.NotFoundError); ok {
				err = nil
			} else {
				return
			}
		}

		start, stop, e := subnet.GetIndexRange()
		if e != nil {
			err = e
			return
		}

		curIp := start
		if vpcIp != nil {
			start = vpcIp.Ip + 1
		}

		for {
			if curIp > stop {
				err = &errortypes.NotFoundError{
					errors.New("vpc: Address pool full"),
				}
				return
			}

			vpcIp = &VpcIp{
				Vpc:      v.Id,
				Subnet:   subId,
				Ip:       curIp,
				Instance: instId,
			}

			_, err = coll.InsertOne(db, vpcIp)
			if err != nil {
				vpcIp = nil
				err = database.ParseError(err)
				if _, ok := err.(*database.DuplicateKeyError); ok {
					err = nil
					curIp += 1
					continue
				}
				return
			}

			break
		}
	}

	instIp, gateIp = vpcIp.GetIps()

	return
}

func (v *Vpc) GetIp6(addr net.IP) net.IP {
	netHash := md5.New()
	netHash.Write(v.Id[:])
	netHashSum := fmt.Sprintf("%x", netHash.Sum(nil))[:12]

	macHash := md5.New()
	macHash.Write(addr)
	macHashSum := fmt.Sprintf("%x", macHash.Sum(nil))[:16]

	ip := fmt.Sprintf("fd97%s%s", netHashSum, macHashSum)
	ipBuf := bytes.Buffer{}

	for i, run := range ip {
		if i%4 == 0 && i != 0 && i != len(ip)-1 {
			ipBuf.WriteRune(':')
		}
		ipBuf.WriteRune(run)
	}

	return net.ParseIP(ipBuf.String())
}

func (v *Vpc) RemoveSubnet(db *database.Database, subId primitive.ObjectID) (
	err error) {

	coll := db.VpcsIp()

	_, err = coll.DeleteMany(db, &bson.M{
		"vpc":    v.Id,
		"subnet": subId,
	})
	if err != nil {
		err = database.ParseError(err)
		return
	}

	return
}

func (v *Vpc) Commit(db *database.Database) (err error) {
	coll := db.Vpcs()

	err = coll.Commit(v.Id, v)
	if err != nil {
		return
	}

	return
}

func (v *Vpc) CommitFields(db *database.Database, fields set.Set) (
	err error) {

	coll := db.Vpcs()

	err = coll.CommitFields(v.Id, v, fields)
	if err != nil {
		return
	}

	return
}

func (v *Vpc) Insert(db *database.Database) (err error) {
	coll := db.Vpcs()

	if !v.Id.IsZero() {
		err = &errortypes.DatabaseError{
			errors.New("vpc: Vpc already exists"),
		}
		return
	}

	resp, err := coll.InsertOne(db, v)
	if err != nil {
		err = database.ParseError(err)
		return
	}

	v.Id = resp.InsertedID.(primitive.ObjectID)

	return
}

func init() {
	module := requires.New("vpc")
	module.After("settings")

	module.Handler = func() (err error) {
		db := database.GetDatabase()
		defer db.Close()

		coll := db.VpcsIp()

		_, err = coll.DeleteMany(db, &bson.M{
			"subnet": nil,
		})
		if err != nil {
			err = database.ParseError(err)
			return
		}

		return
	}
}
