package gorlock

import (
	"github.com/gofrs/uuid"
	"strings"
)

// GetGUID gen random uuid
func GetGUID() (valueGUID string) {
	objID, _ := uuid.NewV4()
	objIdStr := objID.String()
	objIdStr = strings.Replace(objIdStr, "-", "", -1)
	valueGUID = objIdStr
	return valueGUID
}

// SetDebug for debug
func SetDebug() {
	SetLogLevel("DEBUG")
}
