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

const RPCProtocolVersion = 2

type MsgType int32

const (
	Call  MsgType = 0
	Reply MsgType = 1
)

type ReplyStat int32
type RejectStat int32
type AcceptStat int32

const (
	MsgAccepted ReplyStat = 0
	MsgDenied   ReplyStat = 1
)
const (
	Success      AcceptStat = iota // RPC executed successfully
	ProgUnavail                    // Remote hasn't exported the program
	ProgMismatch                   // Remote can't support version number
	ProcUnavail                    // Program can't support procedure
	GarbageArgs                    // Procedure can't decode params
	SystemErr                      // Other errors
)
const (
	RPCMismatch RejectStat = 0 // RPC version number != 2
	AuthError   RejectStat = 1 // Remote can't authenticate caller
)

type ErrRPCMismatch struct {
	Low  uint32
	High uint32
}
type ErrProgMismatch struct {
	Low  uint32
	High uint32
}
type AuthStat int32

const (
	AuthOk AuthStat = iota
	AuthBadcred
	AuthRejectedcred
	AuthBadverf
	AuthRejectedVerf
	AuthTooweak
	AuthInvalidresp
	AuthFailed
)

type AuthFlavor int32

const (
	AuthNone AuthFlavor = iota // No authentication
	AuthSys                    // Unix style
	AuthShort
	AuthDh
	AuthKerb
	AuthRSA
	RPCsecGss
)

type OpaqueAuth struct {
	Flavor AuthFlavor
	Body   []byte
}
type CallBody struct {
	RPCVersion uint32 // must be equal to 2
	Program    uint32
	Version    uint32
	Procedure  uint32
	Cred       OpaqueAuth
	Verf       OpaqueAuth
}
type MismatchReply struct {
	Low  uint32
	High uint32
}
type AcceptedReply struct {
	Verf          OpaqueAuth
	Stat          AcceptStat    `xdr:"union"`
	MismatchInfoP MismatchReply `xdr:"unioncase=1"` // ProgUnavail
	MismatchInfoV MismatchReply `xdr:"unioncase=2"` // ProgMismatch
}
type RejectedReply struct {
	Stat         RejectStat    `xdr:"union"`
	MismatchInfo MismatchReply `xdr:"unioncase=0"` // RPCMismatch
	AuthStat     AuthStat      `xdr:"unioncase=1"` // AuthError
}
type ReplyBody struct {
	Stat   ReplyStat     `xdr:"union"`
	Areply AcceptedReply `xdr:"unioncase=0"`
	Rreply RejectedReply `xdr:"unioncase=1"`
}
type RPCMsg struct {
	Xid   uint32
	Type  MsgType   `xdr:"union"`
	CBody CallBody  `xdr:"unioncase=0"`
	RBody ReplyBody `xdr:"unioncase=1"`
}
type ProcEntry struct {
	Program  uint32   // Remote program
	Version  uint32   // Remote program's version
	ProcList []string // Procedure number
}
