// Copyright © 2016 Asteris, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rpc

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc/metadata"

	"github.com/Sirupsen/logrus"
	"github.com/asteris-llc/converge/apply"
	"github.com/asteris-llc/converge/graph"
	"github.com/asteris-llc/converge/helpers/logging"
	"github.com/asteris-llc/converge/load"
	"github.com/asteris-llc/converge/plan"
	"github.com/asteris-llc/converge/prettyprinters/human"
	"github.com/asteris-llc/converge/render"
	"github.com/asteris-llc/converge/rpc/pb"
	"github.com/fgrid/uuid"
	"github.com/pkg/errors"
)

var (
	errAuthNotProvided = errors.New("authorization not provided")
)

type executor struct {
	auth *authorizer
}

func (e *executor) getLogger(ctx context.Context) (*logrus.Entry, context.Context) {
	logger := getLogger(ctx).WithField("runID", uuid.NewV4().String())

	return logger, logging.WithLogger(ctx, logger)
}

func (e *executor) load(ctx context.Context, location string, params map[string]string) (*graph.Graph, error) {
	logger := getLogger(ctx).WithField("function", "executor.load").WithField("location", location)

	loaded, err := load.Load(ctx, location)
	if err != nil {
		logger.WithError(err).Error("could not load")
		return nil, errors.Wrapf(err, "loading %s", location)
	}

	values := render.Values{}
	for k, v := range params {
		values[k] = v
	}
	rendered, err := render.Render(ctx, loaded, values)
	if err != nil {
		logger.WithError(err).Error("could not render")
		return nil, errors.Wrapf(err, "rendering %s", location)
	}

	merged, err := graph.MergeDuplicates(ctx, rendered, graph.SkipModuleAndParams)
	if err != nil {
		logger.WithError(err).Error("could not merge")
		return nil, errors.Wrapf(err, "merging %s", location)
	}

	return merged, nil
}

type statusResponseStream interface {
	Send(*pb.StatusResponse) error
	SendHeader(metadata.MD) error
}

func (e *executor) edgeMeta(ctx context.Context, g *graph.Graph) (metadata.MD, error) {
	logger := getLogger(ctx).WithField("function", "executor.edgeMeta")

	edges, err := json.Marshal(g.Edges())
	if err != nil {
		logger.WithError(err).Error("could not serialize edges")
		return nil, errors.Wrapf(err, "serializing edges")
	}

	return metadata.New(map[string]string{"edges": string(edges)}), nil
}

func (e *executor) sendMeta(ctx context.Context, g *graph.Graph, stream statusResponseStream) error {
	logger := getLogger(ctx).WithField("function", "executor.sendMeta")

	// dehydrate graph edges and send them in the header metadata
	meta, err := e.edgeMeta(ctx, g)
	if err != nil {
		// already logged, don't log here
		return errors.Wrap(err, "preparing metadata")
	}

	if err = stream.SendHeader(meta); err != nil {
		logger.WithError(err).Error("could not send metadata")
		return errors.Wrap(err, "sending metadata")
	}

	return nil
}

func (e *executor) stageNotifier(stage pb.StatusResponse_Stage, stream statusResponseStream) *graph.Notifier {
	return &graph.Notifier{
		Pre: func(id string) error {
			return stream.Send(&pb.StatusResponse{
				Id:    id,
				Stage: stage,
				Run:   pb.StatusResponse_STARTED,
			})
		},
		Post: func(id string, r interface{}) error {
			response := statusResponseFromPrintable(
				id,
				r.(human.Printable),
				stage,
				pb.StatusResponse_FINISHED,
			)

			return stream.Send(response)
		},
	}
}

func (e *executor) sendPlan(ctx context.Context, stream statusResponseStream, in *graph.Graph) (*graph.Graph, error) {
	out, err := plan.WithNotify(ctx, in, e.stageNotifier(pb.StatusResponse_PLAN, stream))
	if err != nil && err != plan.ErrTreeContainsErrors {
		return nil, err
	}
	return out, nil
}

func (e *executor) Plan(in *pb.ExecRequest, stream pb.Executor_PlanServer) error {
	logger, ctx := e.getLogger(stream.Context())
	logger = logger.WithField("function", "executor.Plan")

	if err := e.auth.authorize(ctx); err != nil {
		logger.WithError(err).Info("authorization failed")
		return errors.Wrap(err, "authorization failed")
	}

	loaded, err := e.load(ctx, in.Location, in.Parameters)
	if err != nil {
		return err
	}

	if err = e.sendMeta(ctx, loaded, stream); err != nil {
		return err
	}

	// send the plan
	_, err = e.sendPlan(ctx, stream, loaded)
	if err != nil {
		logger.WithError(err).WithField("location", in.Location).Error("planning failed")
		return errors.Wrapf(err, "planning %s", in.Location)
	}

	return nil
}

func (e *executor) sendApply(ctx context.Context, stream statusResponseStream, in *graph.Graph) (*graph.Graph, error) {
	out, err := apply.WithNotify(ctx, in, e.stageNotifier(pb.StatusResponse_APPLY, stream))
	if err != nil && err != apply.ErrTreeContainsErrors {
		return nil, err
	}
	return out, nil
}

func (e *executor) Apply(in *pb.ExecRequest, stream pb.Executor_ApplyServer) error {
	logger, ctx := e.getLogger(stream.Context())
	logger = logger.WithField("function", "executor.Apply")

	if err := e.auth.authorize(ctx); err != nil {
		logger.WithError(err).Info("authorization failed")
		return errors.Wrap(err, "authorization failed")
	}

	loaded, err := e.load(ctx, in.Location, in.Parameters)
	if err != nil {
		return err
	}

	if err = e.sendMeta(ctx, loaded, stream); err != nil {
		return err
	}

	planned, err := e.sendPlan(ctx, stream, loaded)
	if err != nil {
		logger.WithError(err).WithField("location", in.Location).Error("planning failed")
		return errors.Wrapf(err, "planning %s", in.Location)
	}

	_, err = e.sendApply(ctx, stream, planned)
	if err != nil {
		logger.WithError(err).WithField("location", in.Location).Error("application failed")
		return errors.Wrapf(err, "applying %s", in.Location)
	}

	return nil
}