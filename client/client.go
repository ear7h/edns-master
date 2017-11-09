/*
this package controls the slave via local http calls
 */
package client

/*
calls

port - returns new port number
register [name] - returns port number
	can be used with other commands ie `testserver $(edns register testserver)
 */


type Request struct {
	Name string `json:"name"`
	Addr string `json:"addr"`
}