//
// Copyright 2021 SkyAPM org
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package sql

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/SkyAPM/go2sky"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

const (
	componentIDUnknown = 0
	componentIDMysql   = 5012
)

const (
	tagDbType          = "db.type"
	tagDbInstance      = "db.instance"
	tagDbStatement     = "db.statement"
	tagDbSqlParameters = "db.sql.parameters"
)

var ErrUnsupportedOp = errors.New("operation unsupported by the underlying driver")

// namedValueToValueString converts driver arguments of NamedValue format to Value string format.
func namedValueToValueString(named []driver.NamedValue) string {
	b := make([]string, 0, len(named))
	for _, param := range named {
		b = append(b, fmt.Sprintf("%v", param.Value))
	}
	return strings.Join(b, ",")
}

// namedValueToValue converts driver arguments of NamedValue format to Value format.
// Implemented in the same way as in database/sql/ctxutil.go.
func namedValueToValue(named []driver.NamedValue) ([]driver.Value, error) {
	dargs := make([]driver.Value, len(named))
	for n, param := range named {
		if len(param.Name) > 0 {
			return nil, errors.New("sql: driver does not support the use of Named Parameters")
		}
		dargs[n] = param.Value
	}
	return dargs, nil
}

func argsToString(args []interface{}) string {
	sb := strings.Builder{}
	for _, arg := range args {
		sb.WriteString(fmt.Sprintf("%v, ", arg))
	}
	return sb.String()
}

func createSpan(ctx context.Context, tracer *go2sky.Tracer, opts *options, operation string) (go2sky.Span, error) {
	s, _, err := tracer.CreateLocalSpan(ctx,
		go2sky.WithSpanType(go2sky.SpanTypeExit),
		go2sky.WithOperationName(opts.getOpName(operation)),
	)
	if err != nil {
		return nil, err
	}
	s.SetPeer(opts.peer)
	s.SetComponent(opts.componentID)
	s.SetSpanLayer(agentv3.SpanLayer_Database)
	s.Tag(tagDbType, string(opts.dbType))
	s.Tag(tagDbInstance, opts.peer)
	return s, nil
}

func createLocalSpan(ctx context.Context, tracer *go2sky.Tracer, opts *options, operation string) (go2sky.Span, context.Context, error) {
	s, nCtx, err := tracer.CreateLocalSpan(ctx,
		go2sky.WithSpanType(go2sky.SpanTypeLocal),
		go2sky.WithOperationName(opts.getOpName(operation)),
	)
	if err != nil {
		return nil, nil, err
	}
	s.SetComponent(opts.componentID)
	s.SetSpanLayer(agentv3.SpanLayer_Database)
	s.Tag(tagDbType, string(opts.dbType))
	s.Tag(tagDbInstance, opts.peer)
	return s, nCtx, nil
}

// parseDsn parse dsn to a endpoint addr string (host:port)
func parseDsn(dbType DBType, dsn string) string {
	var addr string
	switch dbType {
	case MYSQL:
		// [user[:password]@][net[(addr)]]/dbname[?param1=value1&paramN=valueN]
		re := regexp.MustCompile(`\(.+\)`)
		addr = re.FindString(dsn)
		addr = addr[1 : len(addr)-1]
	case IPV4:
		// ipv4 addr
		re := regexp.MustCompile(`((2(5[0-5]|[0-4]\d))|[0-1]?\d{1,2})(\.((2(5[0-5]|[0-4]\d))|[0-1]?\d{1,2})){3}:\d{1,5}`)
		addr = re.FindString(dsn)
	}
	return addr
}
