// Copyright 2021 The Operator-SDK Authors
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

package zap

import (
	"io"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	zapf "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func Logger() logr.Logger {
	return LoggerTo(os.Stderr)
}

func LoggerTo(destWriter io.Writer) logr.Logger {
	conf := getConfig()
	return createLogger(conf, destWriter)
}

func createLogger(conf config, destWriter io.Writer) logr.Logger {
	syncer := zapcore.AddSync(destWriter)

	conf.encoder = &zapf.KubeAwareEncoder{Encoder: conf.encoder, Verbose: conf.level.Level() < 0}
	if conf.sample {
		conf.opts = append(conf.opts, zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewSamplerWithOptions(core, time.Second, 100, 100)
		}))
	}

	conf.opts = append(conf.opts, zap.AddCallerSkip(1), zap.ErrorOutput(syncer))

	log := zap.New(zapcore.NewCore(conf.encoder, syncer, conf.level))
	log = log.WithOptions(conf.opts...)

	return zapr.NewLogger(log)
}

type config struct {
	encoder         zapcore.Encoder
	level           zap.AtomicLevel
	opts            []zap.Option
	stackTraceLevel zapcore.Level
	sample          bool
}

func getConfig() config {
	var c config

	var newEncoder func(...encoderConfigFunc) zapcore.Encoder

	// Set the defaults depending on the log mode (development vs. production)
	if development {
		newEncoder = newConsoleEncoder
		c.level = zap.NewAtomicLevelAt(zap.DebugLevel)
		c.opts = append(c.opts, zap.Development())
		c.sample = false
		c.stackTraceLevel = zap.WarnLevel
	} else {
		newEncoder = newJSONEncoder
		c.level = zap.NewAtomicLevelAt(zap.InfoLevel)
		c.sample = true
		c.stackTraceLevel = zap.ErrorLevel
	}

	// Override the defaults if the flags were set explicitly on the command line
	if stacktraceLevel.set {
		c.stackTraceLevel = stacktraceLevel.level
	}

	c.opts = append(c.opts, zap.AddStacktrace(c.stackTraceLevel))

	var ecfs []encoderConfigFunc

	if encoderVal.set {
		newEncoder = encoderVal.newEncoder
	}

	if timeEncodingVal.set {
		ecfs = append(ecfs, withTimeEncoding(timeEncodingVal.timeEncoder))
	}

	c.encoder = newEncoder(ecfs...)

	if levelVal.set {
		c.level = zap.NewAtomicLevelAt(levelVal.level)
	}

	if sampleVal.set {
		c.sample = sampleVal.sample
	}

	// Disable sampling when we are in debug mode. Otherwise, this will
	// cause index out of bounds errors in the sampling code.
	if c.level.Level() < -1 {
		c.sample = false
	}

	c.opts = append(c.opts, zap.AddCaller())

	return c
}
