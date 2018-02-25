/*
 * Copyright (c) 2018 K. Kumar <storagXpert@gmail.com>
 *
 * This work can be distributed under the terms of the GNU GPLv3.
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License Version 3 for more details.
 */

package oncrpc

import (
	"bytes"
	"fmt"
	"github.com/storagXpert/gonfs/xdr"
	"io"
	"net/rpc"
)

type clientCodec struct {
	conn         io.ReadWriteCloser
	recordReader io.Reader
	procTable    []ProcEntry
}

func NewClientCodec(conn io.ReadWriteCloser, table []ProcEntry) rpc.ClientCodec {
	return &clientCodec{
		conn:      conn,
		procTable: table, //table allocation is callers responsibility
	}
}

func (c *clientCodec) WriteRequest(req *rpc.Request, param interface{}) error {
	pentryIndex := -1
	procIndex := -1
	for i, pentry := range c.procTable {
		for j, proc := range pentry.ProcList {
			if req.ServiceMethod == proc {
				pentryIndex = i
				procIndex = j
				break
			}
		}
	}
	if pentryIndex < 0 || procIndex < 0 {
		return fmt.Errorf("Procedure Unavailable: %s", req.ServiceMethod)
	}
	call := RPCMsg{
		Xid:  uint32(req.Seq),
		Type: Call,
		CBody: CallBody{
			RPCVersion: RPCProtocolVersion,
			Program:    c.procTable[pentryIndex].Program,
			Version:    c.procTable[pentryIndex].Version,
			Procedure:  uint32(procIndex),
		},
	}
	payload := new(bytes.Buffer)
	if _, err := xdr.Marshal(payload, &call); err != nil {
		return err
	}
	if param != nil {
		if _, err := xdr.Marshal(payload, &param); err != nil {
			return err
		}
	}
	if _, err := WriteFullRecord(c.conn, payload.Bytes()); err != nil {
		return err
	}
	return nil
}

func (c *clientCodec) checkReplyForErr(reply *RPCMsg) error {
	if reply.Type != Reply {
		return fmt.Errorf("InvalidRPCMessageType: %d", reply.Type)
	}
	switch reply.RBody.Stat {
	case MsgAccepted:
		switch reply.RBody.Areply.Stat {
		case Success:
		case ProgMismatch:
			return fmt.Errorf("ProgMismatch: low %u high %u", reply.RBody.Areply.MismatchInfoV.Low,
				reply.RBody.Areply.MismatchInfoV.High)
		case ProgUnavail:
			return fmt.Errorf("ProgUnavail: low %u high %u", reply.RBody.Areply.MismatchInfoP.Low,
				reply.RBody.Areply.MismatchInfoP.High)
		case ProcUnavail:
			return fmt.Errorf("ProcUnavail")
		case GarbageArgs:
			return fmt.Errorf("GarbageArgs")
		case SystemErr:
			return fmt.Errorf("SystemErr")
		default:
			return fmt.Errorf("InvalidMsgAccepted")
		}
	case MsgDenied:
		switch reply.RBody.Rreply.Stat {
		case RPCMismatch:
			return fmt.Errorf("RPCMismatch: low %u high %u", reply.RBody.Rreply.MismatchInfo.Low,
				reply.RBody.Rreply.MismatchInfo.High)
		case AuthError:
			return fmt.Errorf("AuthError")
		default:
			return fmt.Errorf("InvalidMsgDeniedType")
		}
	default:
		return fmt.Errorf("InvalidRPCRepyType")
	}
	return nil
}

func (c *clientCodec) ReadResponseHeader(resp *rpc.Response) error {
	record, err := ReadFullRecord(c.conn)
	if err != nil {
		return err
	}
	c.recordReader = bytes.NewReader(record)
	var reply RPCMsg
	if _, err = xdr.Unmarshal(c.recordReader, &reply); err != nil {
		return err
	}
	resp.Seq = uint64(reply.Xid)
	if err = c.checkReplyForErr(&reply); err != nil {
		return err
	}
	return nil
}

func (c *clientCodec) ReadResponseBody(result interface{}) error {
	if result != nil {
		if _, err := xdr.Unmarshal(c.recordReader, &result); err != nil {
			return err
		}
	}
	return nil
}

func (c *clientCodec) Close() error {
	return c.conn.Close()
}
