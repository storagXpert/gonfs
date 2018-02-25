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
	"gonfs/xdr"
	"io"
	"log"
	"net/rpc"
)

type serverCodec struct {
	conn             io.ReadWriteCloser
	recordReader     io.Reader
	procTable        []ProcEntry //Enter Table in increasing order of program number else ProgUnavail would break
	currentProgIndex int         //This is needed by WriteResponse in case of ProgMismatch
}

func NewServerCodec(conn io.ReadWriteCloser, table []ProcEntry) rpc.ServerCodec {
	return &serverCodec{
		conn:             conn,
		currentProgIndex: -1,
		procTable:        table, //table allocation is callers responsibility
	}
}

func (c *serverCodec) ReadRequestHeader(req *rpc.Request) error {

	/* Even If Header has RPC errors we send nil error with req set so that it
	 * could be replayed  back to client */
	record, err := ReadFullRecord(c.conn)
	if err != nil {
		return err
	}
	c.recordReader = bytes.NewReader(record)
	var call RPCMsg
	_, err = xdr.Unmarshal(c.recordReader, &call)
	if err != nil {
		log.Println(err)
		return err
	}
	if call.Type != Call {
		return fmt.Errorf("InvalidRPCMessageType: %d", call.Type)
	}
	req.Seq = uint64(call.Xid)

	if call.CBody.RPCVersion != RPCProtocolVersion {
		log.Printf("NfsProc.RPCMismatch: seq %d, ver %d\n", req.Seq, call.CBody.RPCVersion)
		req.ServiceMethod = "NfsProc.RPCMismatch"
		return nil
	}
	if call.CBody.Cred.Flavor != AuthNone && call.CBody.Cred.Flavor != AuthSys {
		log.Printf("NfsProc.AuthError: seq %d, flavour %d\n", req.Seq, int32(call.CBody.Cred.Flavor))
		req.ServiceMethod = "NfsProc.AuthError"
		return nil
	}
	var programIndex = -1
	for i, pentry := range c.procTable {
		if call.CBody.Program == pentry.Program {
			programIndex = i
			c.currentProgIndex = i
			break
		}
	}
	if programIndex == -1 {
		log.Printf("NfsProc.ProgUnavail: seq %d, program %d\n", req.Seq, call.CBody.Program)
		req.ServiceMethod = "NfsProc.ProgUnavail"
		return nil
	}
	if call.CBody.Version != c.procTable[programIndex].Version {
		log.Printf("NfsProc.ProgMismatch: seq %d, vers %d\n", req.Seq, call.CBody.Version)
		req.ServiceMethod = "NfsProc.ProgMismatch"
		return nil
	}
	if call.CBody.Procedure < 0 ||
		call.CBody.Procedure >= uint32(len(c.procTable[programIndex].ProcList)) {
		log.Printf("NfsProc.ProcUnavail: seq %d, procedure %d\n", req.Seq, call.CBody.Procedure)
		req.ServiceMethod = "NfsProc.ProcUnavail"
		return nil
	}
	/* All is well */
	req.ServiceMethod = c.procTable[programIndex].ProcList[call.CBody.Procedure]
	return nil
}

func (c *serverCodec) ReadRequestBody(funcArgs interface{}) error {
	/* Even if ReadRequestBody returns error, it would be replayed back to client */
	if funcArgs != nil {
		if _, err := xdr.Unmarshal(c.recordReader, &funcArgs); err != nil {
			c.Close()
			log.Printf("NfsProc.GarbageArgs errXDR %v\n", err)
		}
	}
	return nil
}

func (c *serverCodec) WriteResponse(resp *rpc.Response, result interface{}) error {
	reply := RPCMsg{
		Xid:  uint32(resp.Seq),
		Type: Reply,
		RBody: ReplyBody{
			Stat: MsgAccepted,
			Areply: AcceptedReply{
				Stat: Success,
			},
		},
	}
	if resp.Error != "" {
		log.Printf("WriteResponseErr : Method %s Seq %d, Error %s", resp.ServiceMethod, resp.Seq, resp.Error)
	}
	switch resp.ServiceMethod {
	case "NfsProc.ProgUnavail":
		reply.RBody.Areply.MismatchInfoP.Low = c.procTable[0].Program
		reply.RBody.Areply.MismatchInfoP.High = c.procTable[len(c.procTable)-1].Program
		reply.RBody.Areply.Stat = ProgUnavail
	case "NfsProc.ProgMismatch":
		reply.RBody.Areply.MismatchInfoV.Low = c.procTable[c.currentProgIndex].Version
		reply.RBody.Areply.MismatchInfoV.High = c.procTable[c.currentProgIndex].Version
		reply.RBody.Areply.Stat = ProgMismatch
	case "NfsProc.ProcUnavail":
		reply.RBody.Areply.Stat = ProcUnavail
	case "NfsProc.GarbageArgs":
		reply.RBody.Areply.Stat = GarbageArgs
	case "NfsProc.SystemErr":
		reply.RBody.Areply.Stat = SystemErr
	case "NfsProc.RPCMismatch":
		reply.RBody.Rreply.MismatchInfo.Low = uint32(2)
		reply.RBody.Rreply.MismatchInfo.High = uint32(2)
		reply.RBody.Rreply.Stat = RPCMismatch
	case "NfsProc.AuthError":
		reply.RBody.Rreply.AuthStat = AuthBadcred
		reply.RBody.Rreply.Stat = AuthError
	}

	var buf bytes.Buffer
	if _, err := xdr.Marshal(&buf, reply); err != nil {
		c.Close()
		return err
	}
	if _, err := xdr.Marshal(&buf, result); err != nil {
		c.Close()
		return err
	}
	if _, err := WriteFullRecord(c.conn, buf.Bytes()); err != nil {
		c.Close()
		return err
	}
	return nil
}

func (c *serverCodec) Close() error {
	err := c.conn.Close()
	if err != nil {
		log.Printf("connection close failed err %v", err)
	}
	return err
}
