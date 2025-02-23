package v1

import "github.com/mingzaily/maltose/frame/m"

type HelloReq struct {
	m.Meta `method:"GET" path:"/api/v1/hello" summary:"Hello请求"`
	Name   string `form:"name"`
}

type HelloRes struct {
	Name string `json:"name"`
}
