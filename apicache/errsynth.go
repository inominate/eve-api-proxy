package apicache

import (
	"fmt"
	"time"
)

func SynthesizeAPIError(code int, text string, expiresIn time.Duration) []byte {
	errPage := `<eveapi version="2">
<currentTime>%s</currentTime>
<error code="%d">%s</error>
<cachedUntil>%s</cachedUntil>
</eveapi>`

	now := time.Now().UTC().Format(sqlDateTime)
	expires := time.Now().UTC().Add(expiresIn).Format(sqlDateTime)

	return []byte(fmt.Sprintf(errPage, now, code, text, expires))
}
