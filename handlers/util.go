package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/grpc/api"
	"github.com/nicholasjackson/fake-service/logging"
	"github.com/nicholasjackson/fake-service/response"
	"github.com/nicholasjackson/fake-service/worker"
	opentracing "github.com/opentracing/opentracing-go"
	"google.golang.org/grpc/status"
)

func workerHTTP(ctx opentracing.SpanContext, uri string, defaultClient client.HTTP, pr *http.Request, l *logging.Logger) (*response.Response, error) {
	httpReq, _ := http.NewRequest("GET", uri, nil)

	hr := l.CallHTTPUpstream(pr, httpReq, ctx)
	defer hr.Finished()

	code, resp, err := defaultClient.Do(httpReq, pr)

	hr.SetMetadata("response", string(code))
	hr.SetError(err)

	r := &response.Response{}

	if resp != nil {
		jsonerr := r.FromJSON(resp)
		if jsonerr != nil {
			// we can not process the upstream response
			// this could be because the proxy is returning an error not the
			// upstream
			// in this instance create a blank response with the error

		}
	}

	// set the local URI for the upstream
	r.URI = uri
	r.Code = code

	if err != nil {
		r.Error = err.Error()
	}

	return r, err
}

func workerGRPC(ctx opentracing.SpanContext, uri string, grpcClients map[string]client.GRPC, l *logging.Logger) (*response.Response, error) {
	hr, outCtx := l.CallGRCPUpstream(uri, ctx)
	defer hr.Finished()

	c := grpcClients[uri]
	resp, err := c.Handle(outCtx, &api.Nil{})

	r := &response.Response{}
	if err != nil {
		r.Error = err.Error()
		hr.SetError(err) // set the error for logging

		if s, ok := status.FromError(err); ok {
			r.Code = int(s.Code())
			hr.SetMetadata("ResponseCode", string(s.Code())) // set the response code for logging
		}
	}

	if resp != nil {
		jsonerr := r.FromJSON([]byte(resp.Message))
		if jsonerr != nil {
			// we can not process the upstream response
			// this could be because the proxy is returning an error not the
			// upstream
			// in this instance create a blank response with the error
		}
	}

	// set the local URI for the upstream
	r.URI = uri

	if err != nil {
		r.Error = err.Error()
		return r, err
	}

	return r, nil
}

func processResponses(responses []worker.Done) []byte {
	respLines := []string{}

	// append the output from the upstreams
	for _, r := range responses {
		respLines = append(respLines, fmt.Sprintf("## Called upstream uri: %s", r.URI))
		/*
			// indent the reposne from the upstream
			lines := strings.Split(r.Message, "\n")
			for _, l := range lines {
				respLines = append(respLines, fmt.Sprintf("  %s", l))
			}
		*/
	}

	return []byte(strings.Join(respLines, "\n"))
}
