package accounts

import (
	"github.com/launchdarkly/foundation/logger"
	mgo "gopkg.in/mgo.v2"
)

func EnsureIndices(db *mgo.Database) {
	if err := accountIndices(db); err != nil {
		logger.Error.Println("Index creation error: " + err.Error())
		panic("Unable to create mongoDB indices for accounts")
	}

	if err := billingPlanIndices(db); err != nil {
		logger.Error.Println("Index creation error: " + err.Error())
		panic("Unable to create mongoDB indices for billing plans")
	}
}
