package pb

import (
	"fmt"
	"strconv"

	"github.com/panshiqu/golang/utils"
	"google.golang.org/grpc/metadata"
)

func MetadataString(md metadata.MD, k string) (string, error) {
	s := md.Get(k)
	if len(s) == 1 {
		return s[0], nil
	}
	return "", utils.Wrap(fmt.Errorf("%s: %v", k, s))
}

func MetadataInt[T ~int | ~int32 | ~int64](md metadata.MD, k string) (T, error) {
	s, err := MetadataString(md, k)
	if err != nil {
		return 0, utils.Wrap(err)
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, utils.Wrap(err)
	}

	return T(n), nil
}
