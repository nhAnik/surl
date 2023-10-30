package util

import "github.com/sqids/sqids-go"

type Key uint

const JwtClaimsKey Key = iota

var (
	chars  = "1xnXM9kBN6cdYsAvjW3Co7luRePDh8ywaUQ4TStpfH0rqFVK2zimLGIJOgb5ZE"
	length = 8
)

func InitSqid() (*sqids.Sqids, error) {
	return sqids.NewCustom(sqids.Options{
		MinLength: &length,
		Alphabet:  &chars,
	})
}
