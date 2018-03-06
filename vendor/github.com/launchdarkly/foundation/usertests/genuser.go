/*
Some common test-support code that should be used by tests, but not production code. The reason this is not
a _test.go file is because those files are only compiled when the package they are part of is being tested.
So, you can't reference exported functions in antoehr package's _test.go files.
*/
package usertests

import (
	"math/rand"
	"time"

	"github.com/icrowley/fake"
	ld "github.com/launchdarkly/go-client-private"
	"github.com/pborman/uuid"
)

// Lucene/ES special chars: + - && || ! ( ) { } [ ] ^ " ~ * ? : \

var (
	r         = rand.New(rand.NewSource(time.Now().Unix()))
	names     = stringlist{"Brandy Leg's Best-Dang Resale", "Al-Safma Imp, Exp & commercial Agencies!", "grand-electronics-appliances cellulars", "Janets Retail", "Bradd's~Bad Gurl Boutiques and creations", "plusSign + inBetweenWords", "beforeDash-afterDash", "doubleamper && sand", "two || pipes", "bang!", "(parentheses}", "{curly}", "[squarebrackets]", "caret^", "tilde~", "star*", "questionMark?", "colon:", "backslash\\", "quote \" then something else", "dash-dash-dash"}
	starTreks = stringlist{"original", "animated", "the next generation", "deep space nine", "voyager", "enterprise"}
	apples    = stringlist{"red delicious", "golden delicious", "gala", "fuji", "granny smith", "braeburn", "honeycrisp", "cripps pink", "cameo"}

	favoriteNumbers = float64list{33.3, 42.42, 3.14159, 2.71828182}
	booleans        = boollist{true, false}
)

type stringlist []string

func (l stringlist) one() string {
	return l[r.Intn(len(l))]
}

func (l stringlist) some() []interface{} {
	ret := make([]interface{}, r.Intn(len(l)))
	for i := 0; i < len(ret); i++ {
		ret[i] = l.one()
	}
	return ret
}

type float64list []float64

func (l float64list) one() float64 {
	return l[r.Intn(len(l))]
}

func (l float64list) some() []interface{} {
	ret := make([]interface{}, r.Intn(len(l)))
	for i := 0; i < len(ret); i++ {
		ret[i] = l.one()
	}
	return ret
}

type boollist []bool

func (l boollist) one() bool {
	return l[r.Intn(len(l))]
}

func (l boollist) some() []interface{} {
	ret := make([]interface{}, r.Intn(len(l)))
	for i := 0; i < len(ret); i++ {
		ret[i] = l.one()
	}
	return ret
}

func GenUser() ld.User {
	name := fake.FullName()
	return genUserWithKeyAndName(fake.CharactersN(r.Intn(10)+6), &name)
}

func GenUserUUIDKey() ld.User {
	return genUserWithKeyAndName(uuid.New(), &names[r.Intn(len(names))])
}

func GenUserNoName() ld.User {
	return genUserWithKeyAndName(fake.CharactersN(r.Intn(10)+6), nil)
}

func GenUserUUIDKeyNoName() ld.User {
	return genUserWithKeyAndName(uuid.New(), nil)
}

func genUserWithKeyAndName(key string, name *string) ld.User {
	fname := fake.FirstName()
	lname := fake.LastName()
	email := fake.EmailAddress()
	ip := randoString(fake.IPv4, fake.IPv6)

	var fizzles interface{}
	maybeFizzles := starTreks.some()
	if len(maybeFizzles) > 0 {
		fizzles = maybeFizzles
	}
	var luckyNumbers interface{}
	maybeLuckyNumbers := favoriteNumbers.some()
	if len(maybeLuckyNumbers) > 0 {
		luckyNumbers = maybeLuckyNumbers
	}
	return ld.User{
		FirstName: &fname,
		LastName:  &lname,
		Name:      name,
		Email:     &email,
		Key:       &key,
		Ip:        &ip,
		Custom: &map[string]interface{}{
			"bizzle":         starTreks.one(),
			"bazzle":         apples.one(),
			"favoriteNumber": favoriteNumbers.one(),
			"likesCats":      booleans.one(),
			"fizzles":        fizzles,
			"luckyNumbers":   luckyNumbers,
		},
	}
}

// 50% of the time will return a(), 50% of the time will return b()
func randoString(a, b func() string) string {
	switch rando := r.Int31n(2); rando {
	case 0:
		return a()
	case 1:
		return b()
	default:
		panic("shouldn't happen. check your maths.")
	}
}
