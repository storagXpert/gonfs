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

package nfs3

const (
	MNTPATHLEN = 1024
	MNTNAMLEN  = 255
	FHSIZE3    = 64
	/* READDIR3resok size with XDR overhead = 88 bytes attributes, 8 bytes
	   verifier,4 bytes value_follows for first entry, 4 bytes eof flag */
	READDIRRESOKSIZE = 104
	/* entry3 size with XDR overhead = 8 bytes fileid, 4 bytes name length,
	   8 bytes cookie, 4 byte value_follows */
	ENTRY3SIZE           = 24
	tcpMaxData    uint32 = 524288
	udpMaxData    uint32 = 32768
	NFSMAXPATHLEN uint32 = 4096
)

type Fhandle3 []byte
type Name string
type Mountstat3 int32

const (
	MNT3_OK             Mountstat3 = 0
	MNT3ERR_PERM        Mountstat3 = 1
	MNT3ERR_NOENT       Mountstat3 = 2
	MNT3ERR_IO          Mountstat3 = 5
	MNT3ERR_ACCES       Mountstat3 = 13
	MNT3ERR_NOTDIR      Mountstat3 = 20
	MNT3ERR_INVAL       Mountstat3 = 22
	MNT3ERR_NAMETOOLONG Mountstat3 = 63
	MNT3ERR_NOTSUPP     Mountstat3 = 10004
	MNT3ERR_SERVERFAULT Mountstat3 = 10006
)

type Mountlist *Mountbody
type Mountbody struct {
	Ml_hostname  Name
	Ml_directory string
	Ml_next      Mountlist
}

type Groupnode struct {
	Gr_name Name
	Gr_next *Groupnode `xdr:"optional"`
}

type Exports struct {
	Exp_node *Exportnode `xdr:"optional"`
}
type Exportnode struct {
	Ex_dir    string      //Dirpath
	Ex_groups *Groupnode  `xdr:"optional"`
	Ex_next   *Exportnode `xdr:"optional"`
}

const (
	AUTH_NONE int32 = iota
	AUTH_UNIX
)

type Mountres3_ok struct {
	Fhandle Fhandle3
	//	Sized_auth_flavors []int32
	Auth_flavors []int32
}
type Mountres3 struct {
	Fhs_status Mountstat3   `xdr:"union"`
	Mountinfo  Mountres3_ok `xdr:"unioncase=0"` /*  MNT3_OK */
}

const (
	MOUNT_PROGRAM uint32 = 100005
	MOUNT_V3      uint32 = 3
)
const (
	MOUNT3_NULL uint32 = iota
	MOUNT3_MNT
	MOUNT3_DUMP
	MOUNT3_UMNT
	MOUNT3_UMNTALL
	MOUNT3_EXPORT
)

const (
	NFS3_FHSIZE         = 64 /* Maximum bytes in a V3 file handle */
	NFS3_WRITEVERFSIZE  = 8
	NFS3_CREATEVERFSIZE = 8
	NFS3_COOKIEVERFSIZE = 8
)

type Cookieverf3 [NFS3_COOKIEVERFSIZE]byte
type Cookie3 uint64
type Nfs_fh3 struct {
	Data []byte
}
type Filename3 string
type Diropargs3 struct {
	Dir  Nfs_fh3
	Name Filename3
}
type Ftype3 int32

const (
	NF3REG Ftype3 = iota + 1
	NF3DIR
	NF3BLK
	NF3CHR
	NF3LNK
	NF3SOCK
	NF3FIFO
)

type Mode3 uint32
type Uid3 uint32
type Gid3 uint32
type Size3 uint64
type Fileid3 uint64
type Specdata3 struct {
	Specdata1 uint32
	Specdata2 uint32
}
type Nfstime3 struct {
	Seconds  uint32
	Nseconds uint32
}
type Fattr3 struct {
	Type   Ftype3
	Mode   Mode3
	Nlink  uint32
	Uid    Uid3
	Gid    Gid3
	Size   Size3
	Used   Size3
	Rdev   Specdata3
	Fsid   uint64
	Fileid Fileid3
	Atime  Nfstime3
	Mtime  Nfstime3
	Ctime  Nfstime3
}
type Post_op_attr struct {
	Attributes_follow bool   `xdr:"union"`
	Attributes        Fattr3 `xdr:"unioncase=1"`
}
type Nfsstat3 uint32

const (
	NFS3_OK             Nfsstat3 = 0
	NFS3ERR_PERM        Nfsstat3 = 1
	NFS3ERR_NOENT       Nfsstat3 = 2
	NFS3ERR_IO          Nfsstat3 = 5
	NFS3ERR_NXIO        Nfsstat3 = 6
	NFS3ERR_ACCES       Nfsstat3 = 13
	NFS3ERR_EXIST       Nfsstat3 = 17
	NFS3ERR_XDEV        Nfsstat3 = 18
	NFS3ERR_NODEV       Nfsstat3 = 19
	NFS3ERR_NOTDIR      Nfsstat3 = 20
	NFS3ERR_ISDIR       Nfsstat3 = 21
	NFS3ERR_INVAL       Nfsstat3 = 22
	NFS3ERR_FBIG        Nfsstat3 = 27
	NFS3ERR_NOSPC       Nfsstat3 = 28
	NFS3ERR_ROFS        Nfsstat3 = 30
	NFS3ERR_MLINK       Nfsstat3 = 31
	NFS3ERR_NAMETOOLONG Nfsstat3 = 63
	NFS3ERR_NOTEMPTY    Nfsstat3 = 66
	NFS3ERR_DQUOT       Nfsstat3 = 69
	NFS3ERR_STALE       Nfsstat3 = 70
	NFS3ERR_REMOTE      Nfsstat3 = 71
	NFS3ERR_BADHANDLE   Nfsstat3 = 10001
	NFS3ERR_NOT_SYNC    Nfsstat3 = 10002
	NFS3ERR_BAD_COOKIE  Nfsstat3 = 10003
	NFS3ERR_NOTSUPP     Nfsstat3 = 10004
	NFS3ERR_TOOSMALL    Nfsstat3 = 10005
	NFS3ERR_SERVERFAULT Nfsstat3 = 10006
	NFS3ERR_BADTYPE     Nfsstat3 = 10007
	NFS3ERR_JUKEBOX     Nfsstat3 = 10008
)

type Stable_how int32

const (
	UNSTABLE Stable_how = iota
	DATA_SYNC
	FILE_SYNC
)

type Offset3 uint64
type Count3 uint32
type Wcc_attr struct {
	Size  Size3
	Mtime Nfstime3
	Ctime Nfstime3
}
type Pre_op_attr struct {
	Attributes_follow bool     `xdr:"union"`
	Attributes        Wcc_attr `xdr:"unioncase=1"`
}
type Wcc_data struct {
	Before Pre_op_attr
	After  Post_op_attr
}
type WRITE3args struct {
	File   Nfs_fh3
	Offset Offset3
	Count  Count3
	Stable Stable_how
	Data   []byte
}
type Writeverf3 [NFS3_WRITEVERFSIZE]byte
type WRITE3resok struct {
	File_wcc  Wcc_data
	Count     Count3
	Committed Stable_how
	Verf      Writeverf3
}
type WRITE3resfail struct {
	File_wcc Wcc_data
}
type WRITE3res struct {
	Status  Nfsstat3      `xdr:"union"`
	Resok   WRITE3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail WRITE3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type LOOKUP3args struct {
	What Diropargs3
}

type LOOKUP3resok struct {
	Object         Nfs_fh3
	Obj_attributes Post_op_attr
	Dir_attributes Post_op_attr
}
type LOOKUP3resfail struct {
	Dir_attributes Post_op_attr
}
type LOOKUP3res struct {
	Status  Nfsstat3       `xdr:"union"`
	Resok   LOOKUP3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail LOOKUP3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type COMMIT3args struct {
	File   Nfs_fh3
	Offset Offset3
	Count  Count3
}
type COMMIT3resok struct {
	File_wcc Wcc_data
	Verf     Writeverf3
}
type COMMIT3resfail struct {
	File_wcc Wcc_data
}
type COMMIT3res struct {
	Status  Nfsstat3       `xdr:"union"`
	Resok   COMMIT3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail COMMIT3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}

const (
	ACCESS3_READ uint32 = 1 << iota
	ACCESS3_LOOKUP
	ACCESS3_MODIFY
	ACCESS3_EXTEND
	ACCESS3_DELETE
	ACCESS3_EXECUTE
)

type ACCESS3args struct {
	Object Nfs_fh3
	Access uint32
}
type ACCESS3resok struct {
	Obj_attributes Post_op_attr
	Access         uint32
}
type ACCESS3resfail struct {
	Obj_attributes Post_op_attr
}
type ACCESS3res struct {
	Status  Nfsstat3       `xdr:"union"`
	Resok   ACCESS3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail ACCESS3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type GETATTR3args struct {
	Object Nfs_fh3
}
type GETATTR3resok struct {
	Obj_attributes Fattr3
}
type GETATTR3res struct {
	Status Nfsstat3      `xdr:"union"`
	Resok  GETATTR3resok `xdr:"unioncase=0"` /* NFS3_OK */
}
type Time_how uint32

const (
	DONT_CHANGE Time_how = iota
	SET_TO_SERVER_TIME
	SET_TO_CLIENT_TIME
)

type Set_mode3 struct {
	Set_it bool  `xdr:"union"`
	Mode   Mode3 `xdr:"unioncase=1"`
}
type Set_uid3 struct {
	Set_it bool `xdr:"union"`
	Uid    Uid3 `xdr:"unioncase=1"`
}
type Set_gid3 struct {
	Set_it bool `xdr:"union"`
	Gid    Gid3 `xdr:"unioncase=1"`
}
type Set_size3 struct {
	Set_it bool  `xdr:"union"`
	Size   Size3 `xdr:"unioncase=1"`
}
type Set_atime struct {
	Set_it Time_how `xdr:"union"`
	Atime  Nfstime3 `xdr:"unioncase=2"` /* SET_TO_CLIENT_TIME */
}
type Set_mtime struct {
	Set_it Time_how `xdr:"union"`
	Mtime  Nfstime3 `xdr:"unioncase=2"` /* SET_TO_CLIENT_TIME */
}
type Sattr3 struct {
	Mode  Set_mode3
	Uid   Set_uid3
	Gid   Set_gid3
	Size  Set_size3
	Atime Set_atime
	Mtime Set_mtime
}
type Createmode3 int32

const (
	UNCHECKED Createmode3 = iota
	GUARDED
	EXCLUSIVE
)

type Createverf3 [NFS3_CREATEVERFSIZE]byte
type Createhow3 struct {
	Mode           Createmode3 `xdr:"union"`
	Verf           Createverf3 `xdr:"unioncase=2"`    /* EXCLUSIVE*/
	Obj_attributes Sattr3      `xdr:"unionnotcase=2"` /* if UNCHECKED or GUARDED */
}
type CREATE3args struct {
	Where Diropargs3
	How   Createhow3
}
type Post_op_fh3 struct {
	Handle_follows bool    `xdr:"union"`
	Handle         Nfs_fh3 `xdr:"unioncase=1"`
}
type CREATE3resok struct {
	Obj            Post_op_fh3
	Obj_attributes Post_op_attr
	Dir_wcc        Wcc_data
}
type CREATE3resfail struct {
	Dir_wcc Wcc_data
}
type CREATE3res struct {
	Status  Nfsstat3       `xdr:"union"`
	Resok   CREATE3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail CREATE3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type REMOVE3args struct {
	Object Diropargs3
}
type REMOVE3resok struct {
	Dir_wcc Wcc_data
}
type REMOVE3resfail struct {
	Dir_wcc Wcc_data
}
type REMOVE3res struct {
	Status  Nfsstat3       `xdr:"union"`
	Resok   REMOVE3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail REMOVE3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type READ3args struct {
	File   Nfs_fh3
	Offset Offset3
	Count  Count3
}

type READ3resok struct {
	File_attributes Post_op_attr
	Count           Count3
	Eof             bool
	Data            []byte
}
type READ3resfail struct {
	File_attributes Post_op_attr
}
type READ3res struct {
	Status  Nfsstat3     `xdr:"union"`
	Resok   READ3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail READ3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}

const (
	FSF3_LINK uint32 = uint32(1) << iota
	FSF3_SYMLINK
	FSF3_HOMOGENEOUS
	FSF3_CANSETTIME
)

type FSINFO3args struct {
	Fsroot Nfs_fh3
}
type FSINFO3resok struct {
	Obj_attributes Post_op_attr
	Rtmax          uint32
	Rtpref         uint32
	Rtmult         uint32
	Wtmax          uint32
	Wtpref         uint32
	Wtmult         uint32
	Dtpref         uint32
	Maxfilesize    Size3
	Time_delta     Nfstime3
	Properties     uint32
}
type FSINFO3resfail struct {
	Obj_attributes Post_op_attr
}
type FSINFO3res struct {
	Status  Nfsstat3       `xdr:"union"`
	Resok   FSINFO3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail FSINFO3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type FSSTAT3args struct {
	Fsroot Nfs_fh3
}
type FSSTAT3resok struct {
	Obj_attributes Post_op_attr
	Tbytes         Size3
	Fbytes         Size3
	Abytes         Size3
	Tfiles         Size3
	Ffiles         Size3
	Afiles         Size3
	Invarsec       uint32
}
type FSSTAT3resfail struct {
	Obj_attributes Post_op_attr
}
type FSSTAT3res struct {
	Status  Nfsstat3       `xdr:"union"`
	Resok   FSSTAT3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail FSSTAT3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type PATHCONF3args struct {
	Object Nfs_fh3
}
type PATHCONF3resok struct {
	Obj_attributes   Post_op_attr
	Linkmax          uint32
	Name_max         uint32
	No_trunc         bool
	Chown_restricted bool
	Case_insensitive bool
	Case_preserving  bool
}
type PATHCONF3resfail struct {
	Obj_attributes Post_op_attr
}
type PATHCONF3res struct {
	Status  Nfsstat3         `xdr:"union"`
	Resok   PATHCONF3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail PATHCONF3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type Nfspath3 string

type Symlinkdata3 struct {
	Symlink_attributes Sattr3
	Symlink_data       Nfspath3
}
type SYMLINK3args struct {
	Where   Diropargs3
	Symlink Symlinkdata3
}
type SYMLINK3resok struct {
	Obj            Post_op_fh3
	Obj_attributes Post_op_attr
	Dir_wcc        Wcc_data
}
type SYMLINK3resfail struct {
	Dir_wcc Wcc_data
}
type SYMLINK3res struct {
	Status  Nfsstat3        `xdr:"union"`
	Resok   SYMLINK3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail SYMLINK3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type READLINK3args struct {
	Symlink Nfs_fh3
}

type READLINK3resok struct {
	Symlink_attributes Post_op_attr
	Data               Nfspath3
}

type READLINK3resfail struct {
	Symlink_attributes Post_op_attr
}

type READLINK3res struct {
	Status  Nfsstat3         `xdr:"union"`
	Resok   READLINK3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail READLINK3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type Devicedata3 struct {
	Dev_attributes Sattr3
	Spec           Specdata3
}
type Mknoddata3 struct {
	Type             Ftype3      `xdr:"union"`
	Device1          Devicedata3 `xdr:"unioncase=3"` /* NF3BLK  */
	Device2          Devicedata3 `xdr:"unioncase=4"` /* NF3BLK  */
	Pipe_attributes1 Sattr3      `xdr:"unioncase=6"` /* NF3SOCK  */
	Pipe_attributes2 Sattr3      `xdr:"unioncase=7"` /* NF3FIFO  */
}
type MKNOD3args struct {
	Where Diropargs3
	What  Mknoddata3
}
type MKNOD3resok struct {
	Obj            Post_op_fh3
	Obj_attributes Post_op_attr
	Dir_wcc        Wcc_data
}
type MKNOD3resfail struct {
	Dir_wcc Wcc_data
}
type MKNOD3res struct {
	Status  Nfsstat3      `xdr:"union"`
	Resok   MKNOD3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail MKNOD3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type MKDIR3args struct {
	Where      Diropargs3
	Attributes Sattr3
}
type MKDIR3resok struct {
	Obj            Post_op_fh3
	Obj_attributes Post_op_attr
	Dir_wcc        Wcc_data
}
type MKDIR3resfail struct {
	Dir_wcc Wcc_data
}
type MKDIR3res struct {
	Status  Nfsstat3      `xdr:"union"`
	Resok   MKDIR3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail MKDIR3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type RMDIR3args struct {
	Object Diropargs3
}
type RMDIR3resok struct {
	Dir_wcc Wcc_data
}
type RMDIR3resfail struct {
	Dir_wcc Wcc_data
}
type RMDIR3res struct {
	Status  Nfsstat3      `xdr:"union"`
	Resok   RMDIR3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail RMDIR3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type RENAME3args struct {
	From Diropargs3
	To   Diropargs3
}
type RENAME3resok struct {
	Fromdir_wcc Wcc_data
	Todir_wcc   Wcc_data
}
type RENAME3resfail struct {
	Fromdir_wcc Wcc_data
	Todir_wcc   Wcc_data
}

type RENAME3res struct {
	Status  Nfsstat3       `xdr:"union"`
	Resok   RENAME3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail RENAME3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type READDIRPLUS3args struct {
	Dir        Nfs_fh3
	Cookie     Cookie3
	Cookieverf Cookieverf3
	Dircount   Count3
	Maxcount   Count3
}

type Entryplus3 struct {
	Fileid          Fileid3
	Name            Filename3
	Cookie          Cookie3
	Name_attributes Post_op_attr
	Name_handle     Post_op_fh3
	Nextentry       *Entryplus3
}
type Dirlistplus3 struct {
	Entries *Entryplus3
	Eof     bool
}
type READDIRPLUS3resok struct {
	Dir_attributes Post_op_attr
	Cookieverf     Cookieverf3
	Reply          Dirlistplus3
}
type READDIRPLUS3resfail struct {
	Dir_attributes Post_op_attr
}
type READDIRPLUS3res struct {
	Status  Nfsstat3            `xdr:"union"`
	Resok   READDIRPLUS3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail READDIRPLUS3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type READDIR3args struct {
	Dir        Nfs_fh3
	Cookie     Cookie3
	Cookieverf Cookieverf3
	Count      Count3
}
type Entry3 struct {
	Fileid    Fileid3
	Name      Filename3
	Cookie    Cookie3
	Nextentry *Entry3 `xdr:"optional"`
}
type Dirlist3 struct {
	Entries *Entry3 `xdr:"optional"`
	Eof     bool
}
type READDIR3resok struct {
	Dir_attributes Post_op_attr
	Cookieverf     Cookieverf3
	Reply          Dirlist3
}
type READDIR3resfail struct {
	Dir_attributes Post_op_attr
}
type READDIR3res struct {
	Status  Nfsstat3        `xdr:"union"`
	Resok   READDIR3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail READDIR3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type LINK3args struct {
	File Nfs_fh3
	Link Diropargs3
}
type LINK3resok struct {
	File_attributes Post_op_attr
	Linkdir_wcc     Wcc_data
}
type LINK3resfail struct {
	File_attributes Post_op_attr
	Linkdir_wcc     Wcc_data
}
type LINK3res struct {
	Status  Nfsstat3     `xdr:"union"`
	Resok   LINK3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail LINK3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}
type Sattrguard3 struct {
	Check     bool     `xdr:"union"`
	Obj_ctime Nfstime3 `xdr:"unioncase=1"`
}
type SETATTR3args struct {
	Object         Nfs_fh3
	New_attributes Sattr3
	Guard          Sattrguard3
}
type SETATTR3resok struct {
	Obj_wcc Wcc_data
}
type SETATTR3resfail struct {
	Obj_wcc Wcc_data
}
type SETATTR3res struct {
	Status  Nfsstat3        `xdr:"union"`
	Resok   SETATTR3resok   `xdr:"unioncase=0"`    /* NFS3_OK */
	Resfail SETATTR3resfail `xdr:"unionnotcase=0"` /* if !NFS3_OK */
}

const (
	NFS_PROGRAM uint32 = 100003
	NFS_V3      uint32 = 3
)
const (
	NFS3_NULL uint32 = iota
	NFS3_GETATTR
	NFS3_SETATTR
	NFS3_LOOKUP
	NFS3_ACCESS
	NFS3_READLINK
	NFS3_READ
	NFS3_WRITE
	NFS3_CREATE
	NFS3_MKDIR
	NFS3_SYMLINK
	NFSPROC3_MKNOD
	NFS3_REMOVE
	NFS3_RMDIR
	FS3_RENAME
	NFS3_LINK
	NFS3_READDIR
	NFS3_READDIRPLUS
	NFS3_FSSTAT
	NFS3_FSINFO
	NFS3_PATHCONF
	NFS3_COMMIT /* No 21 */
)

var Mount3ProcList = [...]string{
	0: "NfsProc.Mountproc_null_3_svc",
	1: "NfsProc.Mountproc_mnt_3_svc",
	2: "NfsProc.Mountproc_dump_3_svc",
	3: "NfsProc.Mountproc_umnt_3_svc",
	4: "NfsProc.Mountproc_umntall_3_svc",
	5: "NfsProc.Mountproc_export_3_svc",
}

var Nfs3ProcList = [...]string{
	0:  "NfsProc.Nfsproc3_null_3_svc",
	1:  "NfsProc.Nfsproc3_getattr_3_svc",
	2:  "NfsProc.Nfsproc3_setattr_3_svc",
	3:  "NfsProc.Nfsproc3_lookup_3_svc",
	4:  "NfsProc.Nfsproc3_access_3_svc",
	5:  "NfsProc.Nfsproc3_readlink_3_svc",
	6:  "NfsProc.Nfsproc3_read_3_svc",
	7:  "NfsProc.Nfsproc3_write_3_svc",
	8:  "NfsProc.Nfsproc3_create_3_svc",
	9:  "NfsProc.Nfsproc3_mkdir_3_svc",
	10: "NfsProc.Nfsproc3_symlink_3_svc",
	11: "NfsProc.Nfsproc3_mknod_3_svc",
	12: "NfsProc.Nfsproc3_remove_3_svc",
	13: "NfsProc.Nfsproc3_rmdir_3_svc",
	14: "NfsProc.Nfsproc3_rename_3_svc",
	15: "NfsProc.Nfsproc3_link_3_svc",
	16: "NfsProc.Nfsproc3_readdir_3_svc",
	17: "NfsProc.Nfsproc3_readdirplus_3_svc",
	18: "NfsProc.Nfsproc3_fsstat_3_svc",
	19: "NfsProc.Nfsproc3_fsinfo_3_svc",
	20: "NfsProc.Nfsproc3_pathconf_3_svc",
	21: "NfsProc.Nfsproc3_commit_3_svc",
}
