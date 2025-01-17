package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/errors"
	"github.com/nicholasjackson/fake-service/grpc/api"
	"github.com/nicholasjackson/fake-service/load"
	"github.com/nicholasjackson/fake-service/logging"
	"github.com/nicholasjackson/fake-service/response"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/nicholasjackson/fake-service/worker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FakeServer implements the gRPC interface
type FakeServer struct {
	name          string
	message       string
	duration      *timing.RequestDuration
	upstreamURIs  []string
	workerCount   int
	defaultClient client.HTTP
	grpcClients   map[string]client.GRPC
	errorInjector *errors.Injector
	loadGenerator *load.Generator
	log           *logging.Logger
}

// NewFakeServer creates a new instance of FakeServer
func NewFakeServer(
	name, message string,
	duration *timing.RequestDuration,
	upstreamURIs []string,
	workerCount int,
	defaultClient client.HTTP,
	grpcClients map[string]client.GRPC,
	i *errors.Injector,
	loadGenerator *load.Generator,
	l *logging.Logger,
) *FakeServer {

	return &FakeServer{
		name:          name,
		message:       message,
		duration:      duration,
		upstreamURIs:  upstreamURIs,
		workerCount:   workerCount,
		defaultClient: defaultClient,
		grpcClients:   grpcClients,
		errorInjector: i,
		loadGenerator: loadGenerator,
		log:           l,
	}
}

// Handle implmements the FakeServer Handle interface method
func (f *FakeServer) Handle(ctx context.Context, in *api.Nil) (*api.Response, error) {

	// start timing the service this is used later for the total request time
	ts := time.Now()
	finished := f.loadGenerator.Generate()
	defer finished()

	hq := f.log.HandleGRCPRequest(ctx)
	defer hq.Finished()

	resp := &response.Response{}
	resp.Name = f.name
	resp.Type = "gRPC"

	// are we injecting errors, if so return the error
	if er := f.errorInjector.Do(); er != nil {
		resp.Code = er.Code
		resp.Error = er.Error.Error()

		hq.SetError(er.Error)
		hq.SetMetadata("response", string(er.Code))

		// return the error
		return &api.Response{Message: resp.ToJSON()}, status.New(codes.Code(resp.Code), er.Error.Error()).Err()
	}

	// if we need to create upstream requests create a worker pool
	var upstreamError error
	if len(f.upstreamURIs) > 0 {
		wp := worker.New(f.workerCount, func(uri string) (*response.Response, error) {
			if strings.HasPrefix(uri, "http://") {
				return workerHTTP(hq.Span.Context(), uri, f.defaultClient, nil, f.log)
			}

			return workerGRPC(hq.Span.Context(), uri, f.grpcClients, f.log)
		})

		err := wp.Do(f.upstreamURIs)

		if err != nil {
			upstreamError = err
		}

		for _, v := range wp.Responses() {
			resp.AppendUpstream(v.Response)
		}
	}

	// service time is equal to the randomised time - the current time take
	d := f.duration.Calculate()
	et := time.Now().Sub(ts)
	rd := d - et

	if upstreamError != nil {
		resp.Code = int(codes.Internal)
		resp.Error = upstreamError.Error()

		hq.SetMetadata("response", string(resp.Code))
		hq.SetError(upstreamError)

		return &api.Response{Message: resp.ToJSON()}, status.New(codes.Internal, upstreamError.Error()).Err()
	}

	// randomize the time the request takes
	lp := f.log.SleepService(hq.Span, rd)

	if rd > 0 {
		time.Sleep(rd)
	}

	lp.Finished()

	// log response code
	hq.SetMetadata("response", "0")

	et = time.Now().Sub(ts)
	resp.Duration = et.String()

	// add the response body if there is no upstream error
	if upstreamError == nil {
		resp.Body = f.message
	}

	return &api.Response{Message: resp.ToJSON()}, nil
}
