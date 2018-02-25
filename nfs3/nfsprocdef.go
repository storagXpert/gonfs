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

import (
	"encoding/binary"
)

/* Signature of every RPC method :
- the method is exported.( start with capital Letter )
- the method has two arguments, both exported (or builtin) types.
- the method's second argument is a pointer.
- the method has return type error.
   func (t *T) MethodName(argType T1, replyType *T2) error
*/
type NfsProc int32
type Void struct{}

func (p *NfsProc) ProgUnavail(args Void, reply *Void) error {
	return nil
}
func (p *NfsProc) ProgMismatch(args Void, reply *Void) error {
	return nil
}
func (p *NfsProc) ProcUnavail(args Void, reply *Void) error {
	return nil
}
func (p *NfsProc) GarbageArgs(args Void, reply *Void) error {
	return nil
}
func (p *NfsProc) SystemErr(args Void, reply *Void) error {
	return nil
}
func (p *NfsProc) AuthError(args Void, reply *Void) error {
	return nil
}
func (p *NfsProc) RPCMismatch(args Void, reply *Void) error {
	return nil
}
func (p *NfsProc) Mountproc_null_3_svc(args Void, reply *Void) error {
	return nil
}

func (p *NfsProc) Mountproc_export_3_svc(args Void, reply *Exports) error {
	be := GetBEInstance()
	be.ExportedMountPoint(reply)
	return nil
}

func (p *NfsProc) Mountproc_mnt_3_svc(args string, reply *Mountres3) error {
	be := GetBEInstance()
	reply.Fhs_status = be.Mount(args, reply)
	return nil
}

func (p *NfsProc) Mountproc_dump_3_svc(args Void, reply *Mountlist) error {
	/* not supporting this currently */
	/*
		be := GetBEInstance()
		be.DumpMountList(reply)
	*/
	return nil
}

func (p *NfsProc) Mountproc_umnt_3_svc(args string, reply *Void) error {
	be := GetBEInstance()
	be.UnMount(args)
	return nil
}

func (p *NfsProc) Mountproc_umntall_3_svc(args Void, reply *Void) error {
	return nil
}

func (t *NfsProc) Nfsproc3_null_3_svc(args Void, reply *Void) error {
	return nil
}
func (t *NfsProc) Nfsproc3_getattr_3_svc(argp *GETATTR3args, reply *GETATTR3res) error {
	fHandle := binary.LittleEndian.Uint64(argp.Object.Data)
	be := GetBEInstance()
	reply.Status = be.GetFileAttr(fHandle, &reply.Resok.Obj_attributes)
	return nil
}

func (t *NfsProc) Nfsproc3_setattr_3_svc(argp *SETATTR3args, reply *SETATTR3res) error {
	fHandle := binary.LittleEndian.Uint64(argp.Object.Data)
	be := GetBEInstance()
	reply.Status = be.Setattr(fHandle, argp.New_attributes, argp.Guard, reply)
	return nil
}
func (t *NfsProc) Nfsproc3_lookup_3_svc(argp *LOOKUP3args, reply *LOOKUP3res) error {
	fHandle := binary.LittleEndian.Uint64(argp.What.Dir.Data)
	be := GetBEInstance()
	reply.Status = be.Lookup(fHandle, string(argp.What.Name), reply)
	return nil
}
func (t *NfsProc) Nfsproc3_access_3_svc(argp *ACCESS3args, reply *ACCESS3res) error {
	fHandle := binary.LittleEndian.Uint64(argp.Object.Data)
	be := GetBEInstance()
	reply.Status = be.Access(fHandle, argp.Access, reply)
	return nil
}
func (t *NfsProc) Nfsproc3_readlink_3_svc(argp *READLINK3args, reply *READLINK3res) error {
	linkFHandle := binary.LittleEndian.Uint64(argp.Symlink.Data)
	be := GetBEInstance()
	reply.Status = be.ReadLink(linkFHandle, reply)
	return nil
}
func (t *NfsProc) Nfsproc3_read_3_svc(argp *READ3args, reply *READ3res) error {
	fHandle := binary.LittleEndian.Uint64(argp.File.Data)
	be := GetBEInstance()
	reply.Status = be.Read(fHandle, uint64(argp.Offset), uint32(argp.Count), reply)
	return nil
}
func (t *NfsProc) Nfsproc3_write_3_svc(argp *WRITE3args, reply *WRITE3res) error {
	fHandle := binary.LittleEndian.Uint64(argp.File.Data)
	be := GetBEInstance()
	reply.Status = be.Write(fHandle, uint64(argp.Offset), uint32(argp.Count), argp.Stable, argp.Data, reply)

	return nil
}
func (t *NfsProc) Nfsproc3_create_3_svc(argp *CREATE3args, reply *CREATE3res) error {
	dirFHandle := binary.LittleEndian.Uint64(argp.Where.Dir.Data)
	fileName := string(argp.Where.Name)
	be := GetBEInstance()
	reply.Status = be.Create(dirFHandle, fileName, argp.How, reply)
	return nil
}
func (t *NfsProc) Nfsproc3_mkdir_3_svc(argp *MKDIR3args, reply *MKDIR3res) error {
	fHandle := binary.LittleEndian.Uint64(argp.Where.Dir.Data)
	be := GetBEInstance()
	reply.Status = be.Mkdir(fHandle, string(argp.Where.Name), &argp.Attributes, reply)
	return nil
}
func (t *NfsProc) Nfsproc3_symlink_3_svc(argp *SYMLINK3args, reply *SYMLINK3res) error {
	dirFHandle := binary.LittleEndian.Uint64(argp.Where.Dir.Data)
	be := GetBEInstance()
	reply.Status = be.SymLink(dirFHandle, string(argp.Where.Name), string(argp.Symlink.Symlink_data),
		argp.Symlink.Symlink_attributes, reply)
	return nil
}
func (t *NfsProc) Nfsproc3_mknod_3_svc(argp *MKNOD3args, reply *MKNOD3res) error {
	reply.Status = NFS3ERR_NOTSUPP
	return nil
}
func (t *NfsProc) Nfsproc3_remove_3_svc(argp *REMOVE3args, reply *REMOVE3res) error {
	fHandle := binary.LittleEndian.Uint64(argp.Object.Dir.Data)
	be := GetBEInstance()
	reply.Status = be.Remove(fHandle, string(argp.Object.Name), reply)
	return nil
}
func (t *NfsProc) Nfsproc3_rmdir_3_svc(argp *RMDIR3args, reply *RMDIR3res) error {
	fHandle := binary.LittleEndian.Uint64(argp.Object.Dir.Data)
	be := GetBEInstance()
	reply.Status = be.Rmdir(fHandle, string(argp.Object.Name), reply)
	return nil
}
func (t *NfsProc) Nfsproc3_rename_3_svc(argp *RENAME3args, reply *RENAME3res) error {
	fromFHandle := binary.LittleEndian.Uint64(argp.From.Dir.Data)
	fromName := string(argp.From.Name)
	toFHandle := binary.LittleEndian.Uint64(argp.To.Dir.Data)
	toName := string(argp.To.Name)
	be := GetBEInstance()
	reply.Status = be.Rename(fromFHandle, fromName, toFHandle, toName, reply)
	return nil
}
func (t *NfsProc) Nfsproc3_link_3_svc(argp *LINK3args, reply *LINK3res) error {
	oldFHandle := binary.LittleEndian.Uint64(argp.File.Data)
	linkDirFHandle := binary.LittleEndian.Uint64(argp.Link.Dir.Data)
	newName := string(argp.Link.Name)
	be := GetBEInstance()
	reply.Status = be.Link(oldFHandle, linkDirFHandle, newName, reply)
	return nil
}
func (t *NfsProc) Nfsproc3_readdir_3_svc(argp *READDIR3args, reply *READDIR3res) error {
	fHandle := binary.LittleEndian.Uint64(argp.Dir.Data)
	be := GetBEInstance()
	reply.Status = be.ReadDir(fHandle, argp.Cookie, argp.Count, reply)
	return nil
}
func (t *NfsProc) Nfsproc3_readdirplus_3_svc(argp *READDIRPLUS3args, reply *READDIRPLUS3res) error {
	fHandle := binary.LittleEndian.Uint64(argp.Dir.Data)
	be := GetBEInstance()
	reply.Status = be.ReadDirPlus(fHandle, reply)
	return nil
}
func (t *NfsProc) Nfsproc3_fsstat_3_svc(argp *FSSTAT3args, reply *FSSTAT3res) error {

	fHandle := binary.LittleEndian.Uint64(argp.Fsroot.Data)
	be := GetBEInstance()
	reply.Status = be.GetFileSystemAttr(fHandle, reply)
	return nil
}
func (t *NfsProc) Nfsproc3_fsinfo_3_svc(argp *FSINFO3args, reply *FSINFO3res) error {

	fHandle := binary.LittleEndian.Uint64(argp.Fsroot.Data)
	be := GetBEInstance()
	reply.Status = be.GetFileSysInfo(fHandle, reply)
	return nil
}

func (t *NfsProc) Nfsproc3_pathconf_3_svc(argp *PATHCONF3args, reply *PATHCONF3res) error {
	fHandle := binary.LittleEndian.Uint64(argp.Object.Data)
	be := GetBEInstance()
	reply.Status = be.GetPathConf(fHandle, reply)
	return nil
}
func (t *NfsProc) Nfsproc3_commit_3_svc(argp *COMMIT3args, reply *COMMIT3res) error {
	fHandle := binary.LittleEndian.Uint64(argp.File.Data)
	be := GetBEInstance()
	/* Dont care about offset and count in a file */
	reply.Status = be.Commit(fHandle, reply)
	return nil
}
