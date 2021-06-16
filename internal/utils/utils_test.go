package utils

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"regexp"
	"strconv"
	"testing"
)

func TestRandomString(t *testing.T) {
	tc := []int{
		0,
		10,
		20,
	}

	for _, c := range tc {
		t.Run(strconv.Itoa(c), func(t *testing.T) {
			// Valid characters & length
			reg, err := regexp.Compile(fmt.Sprintf(`^[a-z0-9]{%d}$`, c))
			require.NoError(t, err)

			str := RandomString(c)

			require.True(t, reg.MatchString(str), "valid characters only")
		})
	}
}
