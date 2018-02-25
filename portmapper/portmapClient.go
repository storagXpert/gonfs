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

package portmapper

import (
	"errors"
	"fmt"
	"github.com/storagXpert/gonfs/oncrpc"
	"net"
	"net/rpc"
	"strconv"
)

const (
	PMAP_PORT    int    = 111
	PMAP_PROGRAM uint32 = 100000
	PMAP_VER     uint32 = 2
	PMAP_ADDR    string = "127.0.0.1"
	IPPROTO_TCP  uint32 = 6
	IPPROTO_UDP  uint32 = 17
)

type Mapping struct {
	Program  uint32
	Version  uint32
	Protocol uint32
	Port     uint32
}

/* for server implementation, define PmapProc as type int with proc interfaces */
var PmapProcList = [...]string{
	0: "PmapProc.PmapProc2_null",
	1: "PmapProc.PmapProc2_set",
	2: "PmapProc.PmapProc2_unset",
	3: "PmapProc.PmapProc2_getport",
	4: "PmapProc.PmapProc2_dump",
	5: "PmapProc.PmapProc2_callit",
}

var procTableClient = [...]oncrpc.ProcEntry{
	oncrpc.ProcEntry{
		PMAP_PROGRAM,
		PMAP_VER,
		PmapProcList[:],
	},
}

const (
	PMAPPROC_NULL uint32 = iota
	PMAPPROC_SET
	PMAPPROC_UNSET
	PMAPPROC_GETPORT
	PMAPPROC_DUMP
	PMAPPROC_CALLIT /* No 5 */
)

func getNewPmapRpcClient() (*rpc.Client, error) {
	conn, err := net.Dial("tcp", PMAP_ADDR+":"+strconv.Itoa(PMAP_PORT))
	if err != nil {
		return nil, fmt.Errorf("could not connect to rpcbind service at %s: %d",
			PMAP_ADDR, PMAP_PORT)
	}
	return rpc.NewClientWithCodec(oncrpc.NewClientCodec(conn, procTableClient[:])), nil
}

func PmapSet(programNumber, programVersion uint32, port uint32) (bool, error) {
	var result bool
	client, err := getNewPmapRpcClient()
	if err != nil {
		return result, err
	}
	if client == nil {
		return result, errors.New("Could not create pmap client")
	}
	defer client.Close()
	mapping := &Mapping{
		Program:  programNumber,
		Version:  programVersion,
		Protocol: IPPROTO_TCP,
		Port:     port,
	}
	err = client.Call("PmapProc.PmapProc2_set", mapping, &result)
	return result, err
}

func PmapUnset(programNumber, programVersion uint32) (bool, error) {
	var result bool
	client, err := getNewPmapRpcClient()
	if err != nil {
		return result, err
	}
	if client == nil {
		return result, errors.New("Could not create pmap client")
	}
	defer client.Close()
	mapping := &Mapping{
		Program: programNumber,
		Version: programVersion,
	}
	err = client.Call("PmapProc.PmapProc2_unset", mapping, &result)
	return result, err
}
