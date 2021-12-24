package call

import (
	"github.com/go-resty/resty/v2"
	"github.com/zsyaoo/opencall/pkg/common"
)

type Result struct {
	Data    interface{}
	Code    string
	Message string
}

func NewResuqet() *resty.Request {
	client := resty.New()
	return client.R().EnableTrace()
}

func Call(queryParams map[string]string, token string, url string) *Result {
	request := NewResuqet()
	request.SetQueryParams(queryParams)
	res, _ := request.SetAuthToken(token).Get(url)
	result := Result{}
	if res.StatusCode() == 200 {
		result.Code = common.OK
	}
	return &result
}
