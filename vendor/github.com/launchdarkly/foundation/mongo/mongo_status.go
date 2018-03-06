package mongo

import (
	s "github.com/launchdarkly/foundation/statuschecks"
	"github.com/launchdarkly/foundation/statuschecks/async"
	mgo "gopkg.in/mgo.v2"
	bson "gopkg.in/mgo.v2/bson"
)

func CheckMongo(sess *mgo.Session) s.ServiceStatus {
	db := sess.DB("admin")
	defer db.Session.Close()
	var mongoStatus replSetStatus
	err := db.Run(bson.D{{"replSetGetStatus", 1}}, &mongoStatus)
	if err != nil {
		if err.Error() == "not running with --replSet" {
			return s.DegradedService()
		}
		return s.DownService()
	}
	ret := s.HealthyService()
	for _, member := range mongoStatus.Members {
		if member.Health != 1 {
			ret = s.DegradedService()
		}
	}
	return ret
}

func FutureCheckMongo(sess *mgo.Session) async.FutureServiceStatus {
	return async.MakeFutureServiceStatus(func() s.ServiceStatus {
		return CheckMongo(sess)
	})
}

type replSetStatus struct {
	Members []replSetMemberStatus `json:"members"`
}

type replSetMemberStatus struct {
	Health int `json:"health"`
}
