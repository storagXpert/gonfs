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

package main

import (
	"flag"
	"fmt"
	"github.com/storagXpert/gonfs/nfs3"
	"github.com/storagXpert/gonfs/oncrpc"
	"github.com/storagXpert/gonfs/portmapper"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/user"
	"strconv"
	"strings"
)

func main() {

	var nfsport uint
	var pmapregister bool
	var listenAddr string
	var allowAddr string
	var writeaccess bool
	var exportPath string
	var logFile string

	flag.UintVar(&nfsport, "nfsport", 40389, "port for NFS3 and MOUNT3 service")
	flag.BoolVar(&pmapregister, "pmapregister", true, "true: register with rpcbind running on port 111")
	flag.StringVar(&listenAddr, "listenAddr", "0.0.0.0", "listen on this ip address")
	flag.StringVar(&allowAddr, "allowAddr", "127.0.0.1/24", "allowed client ip")
	flag.BoolVar(&writeaccess, "writeaccess", true, "true : read and write , false : read only")
	flag.StringVar(&exportPath, "exportPath", "", "directory path (Required)")
	flag.StringVar(&logFile, "logFile", "", "logFile path for debug logging")

	flag.Parse()
	/* basic sanity checks */
	if exportPath == "" {
		fmt.Printf("incorrect exportPath\n")
		os.Exit(1)
	}
	if currWorkingDir, err := os.Getwd(); err == nil {
		if strings.HasPrefix(currWorkingDir, exportPath) == true {
			fmt.Printf("currWorkingDir %s cannot be within exportPath %s\n", currWorkingDir, exportPath)
			os.Exit(1)
		}
	} else {
		fmt.Printf("cannot getcurrWorkingDir( err %v)\n", err)
		os.Exit(1)
	}

	_, allowedIpNetwork, err := net.ParseCIDR(allowAddr)
	if err != nil {
		fmt.Printf("incorrect allowAddr\n")
		os.Exit(1)
	}

	/* if pmapregister is set, make sure system has rpcbind installed and started with rpcinfo -p.
	 * we dont cleanup rpc entries when server halts. So run 'sudo rpcinfo -d 100003 3' and
	 * 'sudo rpcinfo -d 100005 3' to remove those entries if you need to run another nfs server */

	if pmapregister == true {
		if _, err := portmapper.PmapUnset(nfs3.MOUNT_PROGRAM, nfs3.MOUNT_V3); err != nil {
			fmt.Printf("PmapUnset failed: %s\nTry running 'rpcinfo -p'\n", err)
			os.Exit(1)
		}
		if _, err := portmapper.PmapSet(nfs3.MOUNT_PROGRAM, nfs3.MOUNT_V3, uint32(nfsport)); err != nil {
			fmt.Printf("PmapSet failed: %s\nTry running 'rpcinfo -p'\n", err)
			os.Exit(1)
		}
		if _, err := portmapper.PmapUnset(nfs3.NFS_PROGRAM, nfs3.NFS_V3); err != nil {
			fmt.Printf("PmapUnset failed: %s\nTry running 'rpcinfo -p'\n", err)
			os.Exit(1)
		}
		if _, err := portmapper.PmapSet(nfs3.NFS_PROGRAM, nfs3.NFS_V3, uint32(nfsport)); err != nil {
			fmt.Printf("PmapSet failed: %s\nTry running 'rpcinfo -p'\n", err)
			os.Exit(1)
		}
	}
	/* setup logging */
	if logFile != "" {
		logFileType, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			fmt.Printf("cannot open logFile: %s", err)
			os.Exit(1)
		}
		defer logFileType.Close()
		log.SetFlags(log.Lshortfile)
		log.SetOutput(logFileType)
	} else {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}
	/* setup Backend */
	be := nfs3.GetBEInstance()
	if err := be.BackendConfigSetup(exportPath, allowAddr, writeaccess); err != nil {
		fmt.Printf("ConfigSetup failed: %s\n", err)
		os.Exit(1)
	}

	/* Server setup operations */
	server := rpc.NewServer()
	NfsProc := new(nfs3.NfsProc)
	server.Register(NfsProc)
	listener, err := net.Listen("tcp", listenAddr+":"+strconv.Itoa(int(nfsport)))
	if err != nil {
		fmt.Printf("net.Listen() failed: %s", err)
		os.Exit(1)
	}
	procTableServer := []oncrpc.ProcEntry{
		oncrpc.ProcEntry{
			nfs3.NFS_PROGRAM,
			nfs3.NFS_V3,
			nfs3.Nfs3ProcList[:],
		},
		oncrpc.ProcEntry{
			nfs3.MOUNT_PROGRAM,
			nfs3.MOUNT_V3,
			nfs3.Mount3ProcList[:],
		},
	}
	/* Star msg and information for Client use */
	usr, err := user.Current()
	if err != nil {
		fmt.Printf("Cannot get current User: %s", err)
	}
	fmt.Printf("\n*****************************************************************\n")
	fmt.Printf("All access would be squashed to user: %s( UID %s, GID %s)\n", usr.Username, usr.Uid, usr.Gid)
	fmt.Printf("Windows10 nfs3 client should have AnonymousUid and AnonymousGid set to( UID %s, GID %s)\n", usr.Uid, usr.Gid)
	fmt.Printf("If server stops before client unmounts, restart it to unmount client\n")
	fmt.Printf("\n*****************************************************************\n")
	fmt.Printf("NFS3 server listening on TCP conn %s.(CTRL+C to stop server)\n\n", listenAddr+":"+strconv.Itoa(int(nfsport)))

	/* loop forever, launching threads for each service request */

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal("listener.Accept() failed: ", err)
		}
		if allowedIpNetwork.Contains(conn.RemoteAddr().(*net.TCPAddr).IP) {
			go server.ServeCodec(oncrpc.NewServerCodec(conn, procTableServer[:]))
		} else {
			log.Printf("Client %s not in allowAddr list", conn.RemoteAddr().String())
		}
	}
}
