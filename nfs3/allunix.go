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
	"errors"
	"hash/fnv"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Backend struct {
	fhandleMap      map[uint64]string
	fhandleRWMutex  sync.RWMutex //protects fhandleMap access
	ServerStartTime int64
	config          configOptions
}
type configOptions struct {
	exportPath     string
	exportPathFid  Fileid3
	exportPathFsid uint64
	allowAddr      string
	writeaccess    bool
}

var instance *Backend
var once sync.Once

func GetBEInstance() *Backend {
	once.Do(func() {
		instance = &Backend{}
		instance.fhandleMap = make(map[uint64]string, 64)
		instance.ServerStartTime = time.Now().Unix()
		log.Printf("NFS3 Backend Initialized: ServerStartTime :%d", instance.ServerStartTime)
	})
	return instance
}

func (be *Backend) makeFHandle(fileName string, fid uint64, fsid uint64) uint64 {
	h := fnv.New64a()
	fidSlice := make([]byte, 8, 8)
	fsidSlice := make([]byte, 8, 8)
	binary.LittleEndian.PutUint64(fidSlice, fid)
	binary.LittleEndian.PutUint64(fsidSlice, fsid)
	h.Write([]byte(fileName))
	h.Write(fidSlice)
	h.Write(fsidSlice)
	return h.Sum64()
}

func (be *Backend) getValuefromHandle(fHandle uint64) (string, bool) {
	be.fhandleRWMutex.RLock()
	value, found := be.fhandleMap[fHandle]
	be.fhandleRWMutex.RUnlock()
	return value, found
}

func (be *Backend) setValueforHandle(fHandle uint64, value string) {
	be.fhandleRWMutex.Lock()
	be.fhandleMap[fHandle] = value
	be.fhandleRWMutex.Unlock()
}

func (be *Backend) deleteHandle(fHandle uint64) {
	be.fhandleRWMutex.Lock()
	delete(be.fhandleMap, fHandle)
	be.fhandleRWMutex.Unlock()
}

/* This function is called directly by main */
func (be *Backend) BackendConfigSetup(exportPath string, allowAddr string, writeAccess bool) error {
	var fattr Fattr3
	if err := be.getFileAttrFromPath(exportPath, &fattr); err == nil {
		if fattr.Type == NF3DIR {
			be.config.exportPath = exportPath
			be.config.exportPathFid = fattr.Fileid
			be.config.exportPathFsid = fattr.Fsid
			be.config.allowAddr = allowAddr
			be.config.writeaccess = writeAccess
		} else {
			return errors.New("exportPath not a directory")
		}
	} else {
		return errors.New("stat failed on exportPath")
	}
	return nil
}
func (be *Backend) ExportedMountPoint(reply *Exports) {
	reply.Exp_node = new(Exportnode)
	reply.Exp_node.Ex_dir = be.config.exportPath
	reply.Exp_node.Ex_groups = new(Groupnode)
	reply.Exp_node.Ex_groups.Gr_name = Name(be.config.allowAddr)
}

func (be *Backend) Mount(mntPoint string, reply *Mountres3) (status Mountstat3) {
	status = MNT3ERR_ACCES
	if mntPoint == be.config.exportPath {
		mountDirFHandle := be.makeFHandle(mntPoint, uint64(be.config.exportPathFid), be.config.exportPathFsid)
		if foundname, found := be.getValuefromHandle(mountDirFHandle); !found {
			be.setValueforHandle(mountDirFHandle, mntPoint)
		} else {
			log.Printf("Mount: ReMount(mountDirFHandle %d, mntPoint %s, foundname %s)",
				mountDirFHandle, mntPoint, foundname)
		}
		reply.Mountinfo.Fhandle = make([]byte, 8, 8)
		binary.LittleEndian.PutUint64(reply.Mountinfo.Fhandle, mountDirFHandle)
		reply.Mountinfo.Auth_flavors = make([]int32, 1, 1)
		reply.Mountinfo.Auth_flavors[0] = AUTH_UNIX
		status = MNT3_OK
	} else {
		log.Printf("Mount: mntPoint %s not exported", mntPoint)
	}
	return status
}

func (be *Backend) UnMount(mntPoint string) {
	if mntPoint == be.config.exportPath {
		mountDirFHandle := be.makeFHandle(mntPoint, uint64(be.config.exportPathFid), be.config.exportPathFsid)
		if _, found := be.getValuefromHandle(mountDirFHandle); found {
			be.deleteHandle(mountDirFHandle)
		} else {
			log.Printf("UnMount: AlreadyUnmounted(mntPoint %s)", mntPoint)
		}
	} else {
		log.Printf("UnMount: mntPoint %s not exported", mntPoint)
	}
}

func (be *Backend) getFileAttrFromPath(fileName string, fattr *Fattr3) (err error) {
	if fileinfo, err := os.Lstat(fileName); err == nil {
		switch mode := fileinfo.Mode(); {
		case mode&os.ModeType == 0:
			fattr.Type = NF3REG
		case mode&os.ModeDir != 0:
			fattr.Type = NF3DIR
		case mode&os.ModeDevice != 0:
			fattr.Type = NF3BLK
		case mode&os.ModeCharDevice != 0:
			fattr.Type = NF3CHR
		case mode&os.ModeSymlink != 0:
			fattr.Type = NF3LNK
		case mode&os.ModeSocket != 0:
			fattr.Type = NF3SOCK
		case mode&os.ModeNamedPipe != 0:
			fattr.Type = NF3FIFO
		}
		fattr.Size = Size3(fileinfo.Size())
		fattr.Mode = Mode3(fileinfo.Mode() & os.ModePerm)
		if stat, ok := fileinfo.Sys().(*syscall.Stat_t); ok {
			fattr.Nlink = uint32(stat.Nlink)
			fattr.Uid = Uid3(stat.Uid)
			fattr.Gid = Gid3(stat.Gid)
			fattr.Used = Size3(stat.Blocks * 512)
			fattr.Rdev.Specdata1 = uint32(stat.Rdev >> 8 & 0xFF)
			fattr.Rdev.Specdata2 = uint32(stat.Rdev & 0xFF)
			fattr.Fsid = stat.Dev
			fattr.Fileid = Fileid3(stat.Ino)
			fattr.Atime.Seconds = uint32(stat.Atim.Sec)
			fattr.Atime.Nseconds = uint32(stat.Atim.Nsec)
			fattr.Mtime.Seconds = uint32(stat.Mtim.Sec)
			fattr.Mtime.Nseconds = uint32(stat.Mtim.Nsec)
			fattr.Ctime.Seconds = uint32(stat.Ctim.Sec)
			fattr.Ctime.Nseconds = uint32(stat.Ctim.Nsec)
		} else {
			log.Printf("getFileAttrFromPath: (err %v)", err)
			return syscall.EIO
		}
	} else {
		log.Printf("getFileAttrFromPath: (err %v)", err)
		return err.(*os.PathError).Err.(syscall.Errno)
	}
	return nil
}

func (be *Backend) GetFileAttr(fHandle uint64, fattr *Fattr3) (status Nfsstat3) {
	status = NFS3ERR_STALE
	if fileName, found := be.getValuefromHandle(fHandle); found {
		if err := be.getFileAttrFromPath(fileName, fattr); err == nil {
			status = NFS3_OK
		} else {
			log.Printf("GetFileAttr: failed on fileName %s,deleting fHandle %d", fileName, fHandle)
			be.deleteHandle(fHandle)
		}
	} else {
		log.Printf("GetFileAttr: fHandle %d not found", fHandle)
	}
	return status
}

func (be *Backend) GetFileSysInfo(fHandle uint64, reply *FSINFO3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	if dirName, found := be.getValuefromHandle(fHandle); found {
		if err := be.getFileAttrFromPath(dirName, &reply.Resok.Obj_attributes.Attributes); err == nil {
			reply.Resok.Obj_attributes.Attributes_follow = true
			reply.Resok.Rtmax = tcpMaxData
			reply.Resok.Dtpref = 4096 //preferred size of a READDIR request
			reply.Resok.Maxfilesize = ^Size3(0)
			reply.Resok.Properties = FSF3_LINK | FSF3_HOMOGENEOUS | FSF3_CANSETTIME | FSF3_SYMLINK
			reply.Resok.Rtmult = 4096
			reply.Resok.Rtpref = tcpMaxData // preferred size of a READ request.
			reply.Resok.Time_delta.Seconds = uint32(1)
			reply.Resok.Wtmax = tcpMaxData
			reply.Resok.Wtmult = 4096
			reply.Resok.Wtpref = tcpMaxData
			status = NFS3_OK
		} else {
			log.Printf("GetFileSysInfo: stat failed on dirName %s deleting fHandle %d", dirName, fHandle)
			be.deleteHandle(fHandle)
		}
	} else {
		log.Printf("GetFileSysInfo: fHandle %d not found", fHandle)
	}
	return status
}

func (be *Backend) GetFileSystemAttr(fHandle uint64, reply *FSSTAT3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	if dirName, found := be.getValuefromHandle(fHandle); found {
		if err := be.getFileAttrFromPath(dirName, &reply.Resok.Obj_attributes.Attributes); err == nil {
			reply.Resok.Obj_attributes.Attributes_follow = true
			var stat syscall.Statfs_t
			if err := syscall.Statfs(dirName, &stat); err == nil {
				reply.Resok.Tbytes = Size3(stat.Blocks * uint64(stat.Frsize)) /* total size of filesystem */
				reply.Resok.Abytes = Size3(stat.Bavail * uint64(stat.Frsize)) /* size available to user */
				reply.Resok.Fbytes = Size3(stat.Bfree * uint64(stat.Frsize))  /* amount of free space */
				reply.Resok.Afiles = Size3(stat.Ffree)                        /* free file slots  available to user */
				reply.Resok.Ffiles = Size3(stat.Ffree)                        /* free file slots in the filesystem  */
				reply.Resok.Tfiles = Size3(stat.Files)                        /* total file slots in the filesystem */
				reply.Resok.Invarsec = uint32(0)                              /* filesystem volatility, 0 =>frequent changes */
				status = NFS3_OK
			} else {
				log.Printf("GetFileSystemAttr: (err %v)", err)
				/* stat dirName for post attributes */
				if err := be.getFileAttrFromPath(dirName, &reply.Resfail.Obj_attributes.Attributes); err == nil {
					reply.Resfail.Obj_attributes.Attributes_follow = true
				} else {
					log.Printf("GetFileSystemAttr: poststat failed on dirName %s deleting fHandle %d", dirName, fHandle)
					be.deleteHandle(fHandle)
				}
			}
		} else {
			log.Printf("GetFileSystemAttr: stat failed on dirName %s deleting fHandle %d", dirName, fHandle)
			be.deleteHandle(fHandle)
		}
	} else {
		log.Printf("GetFileSystemAttr: fHandle %d not found", fHandle)
	}
	return status
}

func (be *Backend) GetPathConf(fHandle uint64, reply *PATHCONF3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	if dirName, found := be.getValuefromHandle(fHandle); found {
		if err := be.getFileAttrFromPath(dirName, &reply.Resok.Obj_attributes.Attributes); err == nil {
			reply.Resok.Obj_attributes.Attributes_follow = true
			reply.Resok.Case_insensitive = false
			reply.Resok.Case_preserving = true
			reply.Resok.Chown_restricted = false
			reply.Resok.Linkmax = ^uint32(0)
			reply.Resok.Name_max = NFSMAXPATHLEN
			reply.Resok.No_trunc = true
			status = NFS3_OK
		} else {
			log.Printf("GetPathConf: stat failed on dirName %s deleting fHandle %d", dirName, fHandle)
			be.deleteHandle(fHandle)
		}
	} else {
		log.Printf("GetPathConf: fHandle %d not found", fHandle)
	}
	return status
}

func getLookupErr(sysError syscall.Errno) (status Nfsstat3) {
	switch {
	case sysError == syscall.ENOENT:
		status = NFS3ERR_NOENT
	case sysError == syscall.EACCES:
		status = NFS3ERR_ACCES
	case sysError == syscall.EINVAL:
		status = NFS3ERR_INVAL
	case sysError == syscall.ENAMETOOLONG:
		status = NFS3ERR_NAMETOOLONG
	case sysError == syscall.ENOTDIR ||
		sysError == syscall.ELOOP:
		status = NFS3ERR_STALE
	default:
		status = NFS3ERR_IO
	}
	return status
}

func (be *Backend) Lookup(fHandle uint64, name string, reply *LOOKUP3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	if dirName, found := be.getValuefromHandle(fHandle); found {
		if err := be.getFileAttrFromPath(dirName, &reply.Resok.Dir_attributes.Attributes); err == nil {
			reply.Resok.Dir_attributes.Attributes_follow = true

			/* check for invalid names */
			if name != "" && strings.ContainsAny(name, "/") == false {
				/* if the Lookup is on exportPath dont allow its parent to be looked up */
				if dirName == be.config.exportPath && name == ".." {
					name = "."
				}
				lookupname := dirName + "/" + name
				if err = be.getFileAttrFromPath(lookupname, &reply.Resok.Obj_attributes.Attributes); err == nil {
					reply.Resok.Obj_attributes.Attributes_follow = true
					newFHandle := be.makeFHandle(lookupname, uint64(reply.Resok.Obj_attributes.Attributes.Fileid),
						reply.Resok.Obj_attributes.Attributes.Fsid)
					if foundname, found := be.getValuefromHandle(newFHandle); !found {
						be.setValueforHandle(newFHandle, lookupname)
					} else {
						/* its possible to find an already existing fHandle during lookup but it should be same as lookup name */
						if foundname != lookupname {
							/* if foundname no longer exists on filesystem, reset the handle with lookupname */
							if _, err := os.Lstat(foundname); err != nil {
								log.Printf("Lookup: resetting staleFHandle(foundname %s, lookupname %s)", foundname, lookupname)
								be.setValueforHandle(newFHandle, lookupname)
							} else {
								log.Fatalf("Lookup: FNV1a collision(foundname %s lookupname %s)", foundname, lookupname)
							}
						}
					}
					reply.Resok.Object.Data = make([]byte, 8, 8)
					binary.LittleEndian.PutUint64(reply.Resok.Object.Data, newFHandle)
					status = NFS3_OK
				} else {
					log.Printf("Lookup: stat failed on lookupname %s", lookupname)
					status = getLookupErr(err.(syscall.Errno))
					/* stat dirName for post attributes */
					if err := be.getFileAttrFromPath(dirName, &reply.Resfail.Dir_attributes.Attributes); err == nil {
						reply.Resfail.Dir_attributes.Attributes_follow = true
					} else {
						log.Printf("Lookup: poststat failed on dirName %s deleting fHandle %d", dirName, fHandle)
						be.deleteHandle(fHandle)
					}
				}
			} else {
				log.Printf("Lookup: unsupported name %s within dirName %s", name, dirName)
				status = NFS3ERR_ACCES
				/* stat dirName for post attributes */
				if err := be.getFileAttrFromPath(dirName, &reply.Resfail.Dir_attributes.Attributes); err == nil {
					reply.Resfail.Dir_attributes.Attributes_follow = true
				} else {
					log.Printf("Lookup: poststat failed on dirName %s deleting fHandle %d", dirName, fHandle)
					be.deleteHandle(fHandle)
				}
			}
		} else {
			log.Printf("Lookup: stat failed on dirName %s, deleting fHandle %d", dirName, fHandle)
			be.deleteHandle(fHandle)
		}
	} else {
		log.Printf("Lookup: fHandle %d not found", fHandle)
	}
	return status
}

func getReadDirErr(sysError syscall.Errno) (status Nfsstat3) {
	switch {
	case sysError == syscall.EPERM:
		status = NFS3ERR_PERM
	case sysError == syscall.EACCES:
		status = NFS3ERR_ACCES
	case sysError == syscall.ENOTDIR:
		status = NFS3ERR_NOTDIR
	case sysError == syscall.ENAMETOOLONG || sysError == syscall.ELOOP ||
		sysError == syscall.ENOENT:
		status = NFS3ERR_STALE
	case sysError == syscall.EINVAL:
		status = NFS3ERR_INVAL
	default:
		status = NFS3ERR_IO
	}
	return status
}
func getDotAndDotDot(dirName string) (dot, dotdot os.FileInfo, err error) {
	if dirName != "" {
		filenameP := dirName + "/" + ".."
		if dotdot, err = os.Lstat(filenameP); err != nil {
			log.Printf("getDotAndDotDot: Lstat failed filenameP %s (err %v)", filenameP, err)
		}
		filenameC := dirName + "/" + "."
		if dot, err = os.Lstat(filenameC); err != nil {
			log.Printf("getDotAndDotDot: Lstat failed filenameC %s (err %v)", filenameC, err)
		}
		return dot, dotdot, err
	} else {
		return nil, nil, errors.New("Empty dirName")
	}
}

func (be *Backend) ReadDir(fHandle uint64, cookie Cookie3, count Count3, reply *READDIR3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	if dirName, found := be.getValuefromHandle(fHandle); found {
		if err := be.getFileAttrFromPath(dirName, &reply.Resok.Dir_attributes.Attributes); err == nil {
			reply.Resok.Dir_attributes.Attributes_follow = true

			if dirFile, err := os.Open(dirName); err == nil {
				defer dirFile.Close()
				if dirEntries, err := dirFile.Readdir(-1); err == nil {
					/*Readdir cannot get dot, dotdot entries */
					var dot, dotdot os.FileInfo
					if dot, dotdot, err = getDotAndDotDot(dirName); err == nil {
						dirEntries = append(dirEntries, dot, dotdot)
					} else {
						log.Printf("ReadDir: dot, dotdot failed", err)
					}

					var dirEntryCount uint64 = uint64(len(dirEntries))
					/* high order 32 bits in cookie keep count of dir entries previously seen and low order
					 * keep count of dir entries already sent plus one.If there is a change in no of entries between invocations,
					 * we log that fact ,for now, and read from next entry as if nothing happened.After that, a lookup
					 * on any removed/changed entry that happed between invocations and we missed it ,will return STALE
					 *  and client should recover
					 */
					prevDirEntryCount := uint64(cookie) >> 32

					if uint64(cookie) != 0 && dirEntryCount != prevDirEntryCount {
						log.Printf("ReadDir: dirEntryCount change on dirName %s(current %d, prev %d)", dirName,
							dirEntryCount, prevDirEntryCount)
					}

					var entrySize uint32 = uint32(READDIRRESOKSIZE)
					/* low order 32 bits of cookie keep count of  dir entries already sent plus one */
					var dirEntryCountSent uint64 = uint64(cookie) & uint64(0xFFFFFFFF)
					var isEof bool = true
					var prevEntryP *Entry3

					for i := dirEntryCountSent; i < dirEntryCount; i++ {
						entryOverhead := uint32(ENTRY3SIZE + ((len(dirEntries[i].Name())+3)/4)*4)
						entrySize += entryOverhead
						if entrySize > uint32(count) {
							/* take care of EOF and return OK since max data limit is reached */
							isEof = false
							break
						} else {
							entryP := new(Entry3)
							entryP.Name = Filename3(dirEntries[i].Name())
							entryP.Fileid = Fileid3(dirEntries[i].Sys().(*syscall.Stat_t).Ino)
							/* we cannot allow parent of ExportPath to be read. load inode of dot in dotdot */
							if dirName == be.config.exportPath && entryP.Name == ".." {
								entryP.Fileid = Fileid3(dot.Sys().(*syscall.Stat_t).Ino)
							}
							entryP.Cookie = Cookie3((dirEntryCount << 32) | (i + 1))

							if i == dirEntryCountSent {
								reply.Resok.Reply.Entries = entryP
							} else {
								prevEntryP.Nextentry = entryP
							}
							prevEntryP = entryP
						}
					}
					reply.Resok.Reply.Eof = isEof
					status = NFS3_OK
				} else {
					log.Printf("ReadDir: readdir failed (err %v)", err)
					status = getReadDirErr(err.(*os.SyscallError).Err.(syscall.Errno))
					/* stat dirName for post attributes */
					if err := be.getFileAttrFromPath(dirName, &reply.Resfail.Dir_attributes.Attributes); err == nil {
						reply.Resfail.Dir_attributes.Attributes_follow = true
					} else {
						log.Printf("ReadDir: poststat failed on dirName %s deleting fHandle %d", dirName, fHandle)
						be.deleteHandle(fHandle)
					}
				}
			} else {
				log.Printf("ReadDir: open failed (err %v)", err)
				status = getReadDirErr(err.(*os.PathError).Err.(syscall.Errno))
				/* stat dirName for post attributes */
				if err := be.getFileAttrFromPath(dirName, &reply.Resfail.Dir_attributes.Attributes); err == nil {
					reply.Resfail.Dir_attributes.Attributes_follow = true
				} else {
					log.Printf("ReadDir: poststat failed on dirName %s deleting fHandle %d", dirName, fHandle)
					be.deleteHandle(fHandle)
				}
			}
		} else {
			log.Printf("ReadDir: stat failed on dirName %s,deleting fHandle %d", dirName, fHandle)
			be.deleteHandle(fHandle)
		}
	} else {
		log.Printf("ReadDir: fHandle %d not found", fHandle)
	}
	return status
}

func (be *Backend) ReadDirPlus(fHandle uint64, reply *READDIRPLUS3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	if dirName, found := be.getValuefromHandle(fHandle); found {
		if err := be.getFileAttrFromPath(dirName, &reply.Resfail.Dir_attributes.Attributes); err == nil {
			/* We dont support readirplus currently */
			reply.Resfail.Dir_attributes.Attributes_follow = true
			status = NFS3ERR_NOTSUPP
		} else {
			log.Printf("ReadDirPlus: stat failed on dirName %s deleting fHandle %d", dirName, fHandle)
			be.deleteHandle(fHandle)
		}
	} else {
		log.Printf("ReadDirPlus: fHandle %d not found", fHandle)
	}
	return status
}

func (be *Backend) Access(fHandle uint64, access uint32, reply *ACCESS3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	if fileName, found := be.getValuefromHandle(fHandle); found {
		if err := be.getFileAttrFromPath(fileName, &reply.Resok.Obj_attributes.Attributes); err == nil {
			reply.Resok.Obj_attributes.Attributes_follow = true
			var access uint32
			if os.Geteuid() == int(reply.Resok.Obj_attributes.Attributes.Uid) {
				if reply.Resok.Obj_attributes.Attributes.Mode&syscall.S_IRUSR != 0 {
					access |= ACCESS3_READ
				}
				if reply.Resok.Obj_attributes.Attributes.Mode&syscall.S_IWUSR != 0 {
					access |= (ACCESS3_MODIFY | ACCESS3_EXTEND)
				}
				if reply.Resok.Obj_attributes.Attributes.Mode&syscall.S_IXUSR != 0 {
					access |= ACCESS3_EXECUTE
				}
			} else if os.Getegid() == int(reply.Resok.Obj_attributes.Attributes.Gid) {
				if reply.Resok.Obj_attributes.Attributes.Mode&syscall.S_IRGRP != 0 {
					access |= ACCESS3_READ
				}
				if reply.Resok.Obj_attributes.Attributes.Mode&syscall.S_IWGRP != 0 {
					access |= (ACCESS3_MODIFY | ACCESS3_EXTEND)
				}
				if reply.Resok.Obj_attributes.Attributes.Mode&syscall.S_IXGRP != 0 {
					access |= ACCESS3_EXECUTE
				}
			} else {
				if reply.Resok.Obj_attributes.Attributes.Mode&syscall.S_IROTH != 0 {
					access |= ACCESS3_READ
				}
				if reply.Resok.Obj_attributes.Attributes.Mode&syscall.S_IWOTH != 0 {
					access |= (ACCESS3_MODIFY | ACCESS3_EXTEND)
				}
				if reply.Resok.Obj_attributes.Attributes.Mode&syscall.S_IXOTH != 0 {
					access |= ACCESS3_EXECUTE
				}
			}
			if reply.Resok.Obj_attributes.Attributes.Type == NF3DIR {
				if access&(ACCESS3_READ|ACCESS3_EXECUTE) != 0 {
					access |= ACCESS3_LOOKUP
				}
				if access&ACCESS3_MODIFY != 0 {
					access |= ACCESS3_DELETE
				}
				access &= ^ACCESS3_EXECUTE
			}
			reply.Resok.Access = access
			status = NFS3_OK
		} else {
			log.Printf("Access: stat failed on fileName %s,deleting fHandle %d", fileName, fHandle)
			be.deleteHandle(fHandle)
		}
	} else {
		log.Printf("Access: fHandle %d not found", fHandle)
	}
	return status
}

func getMkdirErr(sysError syscall.Errno) (status Nfsstat3) {
	switch {
	case sysError == syscall.EACCES || sysError == syscall.EPERM:
		status = NFS3ERR_ACCES
	case sysError == syscall.ENOTDIR:
		status = NFS3ERR_NOTDIR
	case sysError == syscall.EEXIST:
		status = NFS3ERR_EXIST
	case sysError == syscall.ENOSPC:
		status = NFS3ERR_NOSPC
	case sysError == syscall.EROFS:
		status = NFS3ERR_ROFS
	case sysError == syscall.EINVAL:
		status = NFS3ERR_INVAL
	case sysError == syscall.ENAMETOOLONG || sysError == syscall.ELOOP ||
		sysError == syscall.ENOENT:
		status = NFS3ERR_STALE
	default:
		status = NFS3ERR_IO
	}
	return status
}

func (be *Backend) Mkdir(fHandle uint64, dirname string, attribute *Sattr3, reply *MKDIR3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	var fattr Fattr3
	if parentname, found := be.getValuefromHandle(fHandle); found {
		if err := be.getFileAttrFromPath(parentname, &fattr); err == nil {

			preOpAttr := Wcc_attr{Size: fattr.Size, Ctime: fattr.Ctime, Mtime: fattr.Mtime}
			reply.Resok.Dir_wcc.Before.Attributes = preOpAttr
			reply.Resok.Dir_wcc.Before.Attributes_follow = true
			/* preset these values in case we fail */
			reply.Resfail.Dir_wcc.Before.Attributes = preOpAttr
			reply.Resfail.Dir_wcc.Before.Attributes_follow = true

			/* check if export is read-only */
			if be.config.writeaccess == false {
				status = NFS3ERR_ROFS
				reply.Resfail.Dir_wcc.After.Attributes = fattr
				reply.Resfail.Dir_wcc.After.Attributes_follow = true
				return status
			}
			/* check for invalid names: dont allow dot and dotdot  */
			if dirname != "" && strings.ContainsAny(dirname, "/") == false {
				if dirname != "." && dirname != ".." {
					/* we dont set  uid/gid attribute, everything is squashed to euid/egid of current process.
					   Also, the Atime, Mtime is decided by us not by client */
					permissions := os.ModePerm
					if attribute.Mode.Set_it {
						permissions = os.FileMode(attribute.Mode.Mode)
					}
					newDir := parentname + "/" + dirname
					if err = os.Mkdir(newDir, permissions); err == nil {

						if err = be.getFileAttrFromPath(newDir, &reply.Resok.Obj_attributes.Attributes); err == nil {
							reply.Resok.Obj_attributes.Attributes_follow = true
							newFHandle := be.makeFHandle(newDir, uint64(reply.Resok.Obj_attributes.Attributes.Fileid),
								reply.Resok.Obj_attributes.Attributes.Fsid)
							if foundname, found := be.getValuefromHandle(newFHandle); !found {
								be.setValueforHandle(newFHandle, newDir)
							} else {
								/* if foundname no longer exists on filesystem, reset the handle with newDir */
								if foundname != newDir {
									if _, err := os.Lstat(foundname); err != nil {
										log.Printf("Mkdir: resetting staleFHandle(foundname %s, newDir %s)", foundname, newDir)
										be.setValueforHandle(newFHandle, newDir)
									} else {
										log.Fatalf("Mkdir: FNV1a collision(foundname %s newDir %s)", foundname, newDir)
									}
								}
							}
							reply.Resok.Obj.Handle.Data = make([]byte, 8, 8)
							binary.LittleEndian.PutUint64(reply.Resok.Obj.Handle.Data, newFHandle)
							reply.Resok.Obj.Handle_follows = true
							/* stat parentname for post attributes */
							if err = be.getFileAttrFromPath(parentname, &reply.Resok.Dir_wcc.After.Attributes); err == nil {
								reply.Resok.Dir_wcc.After.Attributes_follow = true
							} else {
								/* we just created a directory inside this directory !. still return NFS3_OK */
								log.Printf("Mkdir: poststat failed on parentname %s,deleting fHandle %d", parentname, fHandle)
								reply.Resok.Dir_wcc.After.Attributes_follow = false
								be.deleteHandle(fHandle)
							}
							status = NFS3_OK
						} else {
							/* we just created this directory ! */
							log.Printf("Mkdir: stat failed on newDir %s", newDir, err)
							status = getMkdirErr(err.(syscall.Errno))
							/* stat parentname for post attributes */
							if err = be.getFileAttrFromPath(parentname, &reply.Resfail.Dir_wcc.After.Attributes); err == nil {
								reply.Resfail.Dir_wcc.After.Attributes_follow = true
							} else {
								/* we just created a directory inside this directory ! */
								log.Printf("Mkdir: poststat failed on parentname %s,deleting fHandle %d", parentname, fHandle)
								reply.Resfail.Dir_wcc.After.Attributes_follow = false
								be.deleteHandle(fHandle)
							}
						}
					} else {
						log.Printf("Mkdir: mkdir failed (err %v)", err)
						status = getMkdirErr(err.(*os.PathError).Err.(syscall.Errno))
						/* stat parentname for post attributes */
						if err = be.getFileAttrFromPath(parentname, &reply.Resfail.Dir_wcc.After.Attributes); err == nil {
							reply.Resfail.Dir_wcc.After.Attributes_follow = true
						} else {
							/* we just created a directory inside this directory ! */
							log.Printf("Mkdir: poststat failed on parentname %s,deleting fHandle %d", parentname, fHandle)
							reply.Resfail.Dir_wcc.After.Attributes_follow = false
							be.deleteHandle(fHandle)
						}
					}
				} else {
					log.Printf("Mkdir: unsupported dirname %s within parentname %s", dirname, parentname)
					status = NFS3ERR_EXIST
					/* stat parentname for post attributes */
					if err = be.getFileAttrFromPath(parentname, &reply.Resfail.Dir_wcc.After.Attributes); err == nil {
						reply.Resfail.Dir_wcc.After.Attributes_follow = true
					} else {
						/* we just created a directory inside this directory ! */
						log.Printf("Mkdir: poststat failed on parentname %s,deleting fHandle %d", parentname, fHandle)
						reply.Resfail.Dir_wcc.After.Attributes_follow = false
						be.deleteHandle(fHandle)
					}
				}
			} else {
				log.Printf("Mkdir: unsupported dirname %s within parentname %s", dirname, parentname)
				status = NFS3ERR_ACCES
				/* stat parentname for post attributes */
				if err = be.getFileAttrFromPath(parentname, &reply.Resfail.Dir_wcc.After.Attributes); err == nil {
					reply.Resfail.Dir_wcc.After.Attributes_follow = true
				} else {
					/* we just created a directory inside this directory ! */
					log.Printf("Mkdir: poststat failed on parentname %s,deleting fHandle %d", parentname, fHandle)
					reply.Resfail.Dir_wcc.After.Attributes_follow = false
					be.deleteHandle(fHandle)
				}
			}
		} else {
			log.Printf("Mkdir: stat failed on parentname %s,deleting fHandle %d", parentname, fHandle)
			be.deleteHandle(fHandle)
		}
	} else {
		log.Printf("Mkdir: fHandle %d not found", fHandle)
	}
	return status
}

func getRenameErr(sysError syscall.Errno) (status Nfsstat3) {
	switch {
	case sysError == syscall.EISDIR:
		status = NFS3ERR_ISDIR
	case sysError == syscall.EXDEV:
		status = NFS3ERR_XDEV
	case sysError == syscall.EEXIST:
		status = NFS3ERR_EXIST
	case sysError == syscall.ENOTEMPTY:
		status = NFS3ERR_NOTEMPTY
	case sysError == syscall.EINVAL:
		status = NFS3ERR_INVAL
	case sysError == syscall.ENOTDIR:
		status = NFS3ERR_NOTDIR
	case sysError == syscall.EACCES || sysError == syscall.EPERM:
		status = NFS3ERR_ACCES
	case sysError == syscall.ENOENT:
		status = NFS3ERR_NOENT
	case sysError == syscall.ENAMETOOLONG || sysError == syscall.ELOOP:
		status = NFS3ERR_STALE
	case sysError == syscall.ENOSPC:
		status = NFS3ERR_NOSPC
	case sysError == syscall.EROFS:
		status = NFS3ERR_ROFS
	case sysError == syscall.EMLINK:
		status = NFS3ERR_MLINK
	default:
		status = NFS3ERR_IO
	}
	return status
}

/*
"silly rename" NFS behavior :
Unix type OS semantics allow programs the ability to open a file, unlink it, and still be able to access
the file as long as the file remains open. the NFS Linux client knows when a file has been
unlinked while being opened. When the client sees this unlink operation, it issues an NFS call to
rename the file to a hidden file whose name starts with ".nfs" and ends with a sequence of characters
and digits that guarantees the uniqueness of that special dot file. It remembers the renamed file name.
When the client sees the close operation, it then issues an NFS call to actually delete the .nfs file,
therefore removing its physical storage.

We are going to support the silly business.
*/

func (be *Backend) Rename(fromFHandle uint64, fromName string, toFHandle uint64,
	toName string, reply *RENAME3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	var fattr Fattr3
	if fromDir, found := be.getValuefromHandle(fromFHandle); found {
		if err := be.getFileAttrFromPath(fromDir, &fattr); err == nil {
			preOpAttr := Wcc_attr{Size: fattr.Size, Ctime: fattr.Ctime, Mtime: fattr.Mtime}
			reply.Resok.Fromdir_wcc.Before.Attributes = preOpAttr
			reply.Resok.Fromdir_wcc.Before.Attributes_follow = true
			/* preset these values in case we fail */
			reply.Resfail.Fromdir_wcc.Before.Attributes = preOpAttr
			reply.Resfail.Fromdir_wcc.Before.Attributes_follow = true

			if toDir, found := be.getValuefromHandle(toFHandle); found {
				var toFattr Fattr3
				if err = be.getFileAttrFromPath(toDir, &toFattr); err == nil {
					preOpAttr = Wcc_attr{Size: fattr.Size, Ctime: fattr.Ctime, Mtime: fattr.Mtime}
					reply.Resok.Todir_wcc.Before.Attributes = preOpAttr
					reply.Resok.Todir_wcc.Before.Attributes_follow = true
					/* preset these values in case we fail */
					reply.Resfail.Todir_wcc.Before.Attributes = preOpAttr
					reply.Resfail.Todir_wcc.Before.Attributes_follow = true

					/* check if export is read-only */
					if be.config.writeaccess == false {
						status = NFS3ERR_ROFS
						reply.Resfail.Fromdir_wcc.After.Attributes = fattr
						reply.Resfail.Fromdir_wcc.After.Attributes_follow = true
						reply.Resfail.Todir_wcc.After.Attributes = toFattr
						reply.Resfail.Todir_wcc.After.Attributes_follow = true
						return status
					}
					/* check for invalid names : dont allow dot and dotdot */
					if fromName != "" && toName != "" && strings.ContainsAny(fromName, "/") == false &&
						strings.ContainsAny(toName, "/") == false && fromName != "." && toName != "." &&
						fromName != ".." && toName != ".." {
						from := fromDir + "/" + fromName
						to := toDir + "/" + toName
						if err = os.Rename(from, to); err == nil {
							/* check whether client tried a silly rename */
							if strings.HasPrefix(toName, ".nfs") {
								/* the inode no of .nfs file would be same as "to" after rename.
								Using that inode, get the fHandle currently pointing to "from" and
								reset its value to .nfs file */
								if err := be.getFileAttrFromPath(to, &fattr); err == nil {
									fromFileHandle := be.makeFHandle(from, uint64(fattr.Fileid), fattr.Fsid)
									if foundname, found := be.getValuefromHandle(fromFileHandle); !found {
										/* This shouldnt happen.A client silly renames when  it is already got the handle
										 * to the file(fromFileHandle in this case ) which it does not intend to close immediately */
										log.Printf("Rename : no fromFileHandle found(from %s sillyname %s).Nothing to reset", from, to)
									} else {
										if foundname == from {
											/* After resetting fromFileHandle, operations that send handle would operate on ".nfsXXX" file
											   instead of "from" file until it is deleted. e.g : Read Write Setattr GetFileAttr Access
											   Lookup on "from" file would fail since it uses name instead of handle.Lookup could succeed
											   if "from" file was created or another file was renamed to "from".But in that case, inode
											   would be different and it would be treated as a new file. */

											log.Printf("Rename: resetting fromFileHandle with sillyname(from %s, to %s)", from, to)
											be.setValueforHandle(fromFileHandle, to)
										} else {
											log.Fatalf("Rename: FNV1a collision(foundname %s from %s)", foundname, from)
										}
									}
								} else {
									/* just renamed to this .nfs file but cannot stat it ! */
									log.Printf("Rename: cannot stat .nfs file after sillyrename(from %s, to %s)", from, to)
								}
							}
							/* stat fromDir for post attributes */
							if err = be.getFileAttrFromPath(fromDir, &reply.Resok.Fromdir_wcc.After.Attributes); err == nil {
								reply.Resok.Fromdir_wcc.After.Attributes_follow = true
							} else {
								/* we just renamed from this dir ! */
								log.Printf("Rename: poststat failed on fromDir %s,deleting fromFHandle %d", fromDir, fromFHandle)
								reply.Resok.Fromdir_wcc.After.Attributes_follow = false
								be.deleteHandle(fromFHandle)
							}
							/* stat toDir for post attributes */
							if err = be.getFileAttrFromPath(toDir, &reply.Resok.Todir_wcc.After.Attributes); err == nil {
								reply.Resok.Todir_wcc.After.Attributes_follow = true
							} else {
								/* we just renamed to this dir ! */
								log.Printf("Rename: poststat failed on toDir %sdeleting toFHandle %d", toDir, toFHandle)
								reply.Resok.Todir_wcc.After.Attributes_follow = false
								be.deleteHandle(toFHandle)
							}
							status = NFS3_OK
						} else {
							log.Printf("Rename: rename failed (err %v)", err)
							status = getRenameErr(err.(*os.LinkError).Err.(syscall.Errno))
							/* stat fromDir for post attributes */
							if err = be.getFileAttrFromPath(fromDir, &reply.Resfail.Fromdir_wcc.After.Attributes); err == nil {
								reply.Resfail.Fromdir_wcc.After.Attributes_follow = true
							} else {
								/* we just renamed from this dir ! */
								log.Printf("Rename: poststat failed on fromDir %s,deleting fromFHandle %d", fromDir, fromFHandle)
								reply.Resfail.Fromdir_wcc.After.Attributes_follow = false
								be.deleteHandle(fromFHandle)
							}
							/* stat toDir for post attributes */
							if err = be.getFileAttrFromPath(toDir, &reply.Resfail.Todir_wcc.After.Attributes); err == nil {
								reply.Resfail.Todir_wcc.After.Attributes_follow = true
							} else {
								/* we just renamed to this dir ! */
								log.Printf("Rename: poststat failed on toDir %sdeleting toFHandle %d", toDir, toFHandle)
								reply.Resfail.Todir_wcc.After.Attributes_follow = false
								be.deleteHandle(toFHandle)
							}
						}
					} else {
						log.Printf("Rename: invalid name fromName %s toName %s", fromName, toName)
						status = NFS3ERR_INVAL
						/* stat fromDir for post attributes */
						if err = be.getFileAttrFromPath(fromDir, &reply.Resfail.Fromdir_wcc.After.Attributes); err == nil {
							reply.Resfail.Fromdir_wcc.After.Attributes_follow = true
						} else {
							/* we just renamed from this dir ! */
							log.Printf("Rename: poststat failed on fromDir %s,deleting fromFHandle %d", fromDir, fromFHandle)
							reply.Resfail.Fromdir_wcc.After.Attributes_follow = false
							be.deleteHandle(fromFHandle)
						}
						/* stat toDir for post attributes */
						if err = be.getFileAttrFromPath(toDir, &reply.Resfail.Todir_wcc.After.Attributes); err == nil {
							reply.Resfail.Todir_wcc.After.Attributes_follow = true
						} else {
							/* we just renamed to this dir ! */
							log.Printf("Rename: poststat failed on toDir %sdeleting toFHandle %d", toDir, toFHandle)
							reply.Resfail.Todir_wcc.After.Attributes_follow = false
							be.deleteHandle(toFHandle)
						}
					}
				} else {
					log.Printf("Rename: stat failed on toDir %s,deleting toFHandle %d", toDir, toFHandle)
					be.deleteHandle(toFHandle)
					/* stat fromDir for post attributes */
					if err = be.getFileAttrFromPath(fromDir, &reply.Resfail.Fromdir_wcc.After.Attributes); err == nil {
						reply.Resfail.Fromdir_wcc.After.Attributes_follow = true
					} else {
						/* we just renamed from this dir ! */
						log.Printf("Rename: poststat failed on fromDir %s,deleting fromFHandle %d", fromDir, fromFHandle)
						reply.Resfail.Fromdir_wcc.After.Attributes_follow = false
						be.deleteHandle(fromFHandle)
					}
				}
			} else {
				log.Printf("Rename: toFHandle %d not found", toFHandle)
				/* stat fromDir for post attributes */
				if err = be.getFileAttrFromPath(fromDir, &reply.Resfail.Fromdir_wcc.After.Attributes); err == nil {
					reply.Resfail.Fromdir_wcc.After.Attributes_follow = true
				} else {
					/* we just renamed from this dir ! */
					log.Printf("Rename: poststat failed on fromDir %s,deleting fromFHandle %d", fromDir, fromFHandle)
					reply.Resfail.Fromdir_wcc.After.Attributes_follow = false
					be.deleteHandle(fromFHandle)
				}
			}
		} else {
			log.Printf("Rename: stat failed on fromDir %s,deleting fromFHandle %d", fromDir, fromFHandle)
			be.deleteHandle(fromFHandle)
			/* check if toDir exists for post failure stats */
			if toDir, found := be.getValuefromHandle(toFHandle); found {
				if err = be.getFileAttrFromPath(toDir, &fattr); err == nil {
					preOpAttr := Wcc_attr{Size: fattr.Size, Ctime: fattr.Ctime, Mtime: fattr.Mtime}
					reply.Resfail.Todir_wcc.Before.Attributes = preOpAttr
					reply.Resfail.Todir_wcc.Before.Attributes_follow = true
					reply.Resfail.Todir_wcc.After.Attributes = fattr
					reply.Resfail.Todir_wcc.After.Attributes_follow = true
				} else {
					log.Printf("Rename: stat failed on toDir %s,deleting toFHandle %d", toDir, toFHandle)
					be.deleteHandle(toFHandle)
				}
			} else {
				log.Printf("Rename: toFHandle %d not found", toFHandle)
			}
		}
	} else {
		log.Printf("Rename: fromFHandle %d not found", fromFHandle)
		/* check if toDir exists for post failure stats */
		if toDir, found := be.getValuefromHandle(toFHandle); found {
			if err := be.getFileAttrFromPath(toDir, &fattr); err == nil {
				preOpAttr := Wcc_attr{Size: fattr.Size, Ctime: fattr.Ctime, Mtime: fattr.Mtime}
				reply.Resfail.Todir_wcc.Before.Attributes = preOpAttr
				reply.Resfail.Todir_wcc.Before.Attributes_follow = true
				reply.Resfail.Todir_wcc.After.Attributes = fattr
				reply.Resfail.Todir_wcc.After.Attributes_follow = true
			} else {
				log.Printf("Rename: stat failed on toDir %s,deleting toFHandle %d", toDir, toFHandle)
				be.deleteHandle(toFHandle)
			}
		} else {
			log.Printf("Rename: toFHandle %d not found", toFHandle)
		}
	}
	return status
}

func getCreateErr(sysError syscall.Errno) (status Nfsstat3) {
	switch {

	case sysError == syscall.EEXIST:
		status = NFS3ERR_EXIST
	case sysError == syscall.EACCES:
		status = NFS3ERR_ACCES
	case sysError == syscall.ENAMETOOLONG || sysError == syscall.ELOOP ||
		sysError == syscall.ENOTDIR:
		status = NFS3ERR_STALE
	case sysError == syscall.ENOSPC:
		status = NFS3ERR_NOSPC
	case sysError == syscall.EROFS:
		status = NFS3ERR_ROFS
	default:
		status = NFS3ERR_IO
	}
	return status
}
func (be *Backend) Create(dirFHandle uint64, name string, createHow Createhow3, reply *CREATE3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	var fattr Fattr3
	if dirName, found := be.getValuefromHandle(dirFHandle); found {
		if err := be.getFileAttrFromPath(dirName, &fattr); err == nil {
			preOpAttr := Wcc_attr{Size: fattr.Size, Ctime: fattr.Ctime, Mtime: fattr.Mtime}
			reply.Resok.Dir_wcc.Before.Attributes = preOpAttr
			reply.Resok.Dir_wcc.Before.Attributes_follow = true
			/* preset these values in case we fail */
			reply.Resfail.Dir_wcc.Before.Attributes = preOpAttr
			reply.Resfail.Dir_wcc.Before.Attributes_follow = true

			/* check if export is read-only */
			if be.config.writeaccess == false {
				status = NFS3ERR_ROFS
				reply.Resfail.Dir_wcc.After.Attributes = fattr
				reply.Resfail.Dir_wcc.After.Attributes_follow = true
				return status
			}

			/* check for invalid names: dont allow dot and dotdot */
			if name != "" && strings.ContainsAny(name, "/") == false && name != "." && name != ".." {
				fileName := dirName + "/" + name
				/* for pipes add  for *nix env */
				openFlags := os.O_CREATE | os.O_RDWR | os.O_TRUNC | syscall.O_NONBLOCK
				if createHow.Mode != UNCHECKED {
					openFlags |= os.O_EXCL
				}
				/* The idea behind an 8 byte verifier with EXCLUSIVE mode is that in case the server reboots and the client
				 * retries to create the same file exclusively, we should check for the verfier with the previous one and
				 * if equal, return success instead of NFS3ERR_EXIST.The client fires a follow on request to set the attributes
				 * of the file.
				 * Currently, we dont persist file handles if server crashes and we use tcp ,so no support for the verifier
				 */
				permissions := os.FileMode(0666)
				if (createHow.Mode == GUARDED || createHow.Mode == UNCHECKED) && createHow.Obj_attributes.Mode.Set_it {
					permissions = os.FileMode(createHow.Obj_attributes.Mode.Mode)
				}
				if fType, err := os.OpenFile(fileName, openFlags, permissions); err == nil {
					defer fType.Close()
					if err = be.getFileAttrFromPath(fileName, &reply.Resok.Obj_attributes.Attributes); err == nil {

						reply.Resok.Obj_attributes.Attributes_follow = true
						newFHandle := be.makeFHandle(fileName, uint64(reply.Resok.Obj_attributes.Attributes.Fileid),
							reply.Resok.Obj_attributes.Attributes.Fsid)
						if foundname, found := be.getValuefromHandle(newFHandle); !found {
							be.setValueforHandle(newFHandle, fileName)
						} else {
							/* we could find an existing newFHandle during Create for UNCHECKED mode
							if foundname no longer exists on filesystem, reset the handle with fileName */
							if foundname != fileName {
								if _, err := os.Lstat(foundname); err != nil {
									log.Printf("Create: resetting staleFHandle(foundname %s, fileName %s)", foundname, fileName)
									be.setValueforHandle(newFHandle, fileName)
								} else {
									log.Fatalf("Create: FNV1a collision(foundname %s fileName %s)", foundname, fileName)
								}
							}
						}
						reply.Resok.Obj.Handle.Data = make([]byte, 8, 8)
						binary.LittleEndian.PutUint64(reply.Resok.Obj.Handle.Data, newFHandle)
						reply.Resok.Obj.Handle_follows = true

						/* stat dirName for post attributes */
						if err = be.getFileAttrFromPath(dirName, &reply.Resok.Dir_wcc.After.Attributes); err == nil {
							reply.Resok.Dir_wcc.After.Attributes_follow = true
						} else {
							/* we just created a file in this dir ! */
							log.Printf("Create: poststat failed on dirName %s,deleting dirFHandle %d", dirName, dirFHandle)
							be.deleteHandle(dirFHandle)
						}
						status = NFS3_OK
					} else {
						/* we just created this file! */
						log.Printf("Create: failed stat on fileName %s", fileName)
						status = getCreateErr(err.(syscall.Errno))
						/* stat dirName for post attributes */
						if err = be.getFileAttrFromPath(dirName, &reply.Resfail.Dir_wcc.After.Attributes); err == nil {
							reply.Resfail.Dir_wcc.After.Attributes_follow = true
						} else {
							/* we just created a file in this dir ! */
							log.Printf("Create: poststat failed on dirName %s,deleting dirFHandle %d", dirName, dirFHandle)
							be.deleteHandle(dirFHandle)
						}
					}
				} else {
					log.Printf("Create: open failed on fileName %s", fileName)
					status = getCreateErr(err.(*os.PathError).Err.(syscall.Errno))
					/* stat dirName for post attributes */
					if err = be.getFileAttrFromPath(dirName, &reply.Resfail.Dir_wcc.After.Attributes); err == nil {
						reply.Resfail.Dir_wcc.After.Attributes_follow = true
					} else {
						/* we just tried creating a file in this dir ! */
						log.Printf("Create: poststat failed on dirName %s,deleting dirFHandle %d", dirName, dirFHandle)
						be.deleteHandle(dirFHandle)
					}
				}
			} else {
				log.Printf("Create: invalid name %s", name)
				status = NFS3ERR_INVAL
				/* stat dirName for post attributes */
				if err = be.getFileAttrFromPath(dirName, &reply.Resfail.Dir_wcc.After.Attributes); err == nil {
					reply.Resfail.Dir_wcc.After.Attributes_follow = true
				} else {
					/* we just stated this dir ! */
					log.Printf("Create: poststat failed on dirName %s,deleting dirFHandle %d", dirName, dirFHandle)
					be.deleteHandle(dirFHandle)
				}
			}
		} else {
			log.Printf("Create: stat failed on dirName %s ,deleting dirFHandle %d", dirName, dirFHandle)
			be.deleteHandle(dirFHandle)
		}
	} else {
		log.Printf("Create: dirFHandle %d not found", dirFHandle)
	}
	return status
}

func getSetattrErr(sysError syscall.Errno) (status Nfsstat3) {
	switch {
	case sysError == syscall.EACCES:
		status = NFS3ERR_ACCES
	case sysError == syscall.EPERM:
		status = NFS3ERR_PERM
	case sysError == syscall.ENAMETOOLONG || sysError == syscall.ELOOP ||
		sysError == syscall.ENOTDIR || sysError == syscall.ENOENT:
		status = NFS3ERR_STALE
	case sysError == syscall.EROFS:
		status = NFS3ERR_ROFS
	case sysError == syscall.EINVAL:
		status = NFS3ERR_INVAL
	default:
		status = NFS3ERR_IO
	}
	return status
}
func (be *Backend) Setattr(fHandle uint64, newattr Sattr3, guard Sattrguard3, reply *SETATTR3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	var fattr Fattr3
	if fileName, found := be.getValuefromHandle(fHandle); found {
		if err := be.getFileAttrFromPath(fileName, &fattr); err == nil {
			preOpAttr := Wcc_attr{Size: fattr.Size, Ctime: fattr.Ctime, Mtime: fattr.Mtime}
			reply.Resok.Obj_wcc.Before.Attributes = preOpAttr
			reply.Resok.Obj_wcc.Before.Attributes_follow = true
			/* preset these values in case we fail */
			reply.Resfail.Obj_wcc.Before.Attributes = preOpAttr
			reply.Resfail.Obj_wcc.Before.Attributes_follow = true

			/* check if export is read-only */
			if be.config.writeaccess == false {
				status = NFS3ERR_ROFS
				reply.Resfail.Obj_wcc.After.Attributes = fattr
				reply.Resfail.Obj_wcc.After.Attributes_follow = true
				return status
			}
			if guard.Check == false || guard.Obj_ctime.Seconds == preOpAttr.Ctime.Seconds {
				runOnce := true
				for runOnce {
					/* change file size */
					if newattr.Size.Set_it {
						if err := os.Truncate(fileName, int64(newattr.Size.Size)); err != nil {
							log.Printf("Setattr: truncate failed (err %v)", err)
							status = getSetattrErr(err.(*os.PathError).Err.(syscall.Errno))
							break
						}
					}
					/* change ownership  */
					uid := fattr.Uid
					gid := fattr.Gid
					if newattr.Uid.Set_it {
						uid = newattr.Uid.Uid
					}
					if newattr.Gid.Set_it {
						gid = newattr.Gid.Gid
					}
					if newattr.Uid.Set_it || newattr.Gid.Set_it {
						/* Currently, we squash all uid/gid to euid/egid of server.So dont do anything here */
						log.Printf("Setattr:Attempt to set uid %d gid %d on fileName", uid, gid, fileName)
						/*
							if err := os.Chown(fileName, int(uid), int(gid)); err != nil {
								log.Printf("Setattr: chown failed on fileName %s(err %v)", fileName, err)
								status = getSetattrErr(err.(*os.PathError).Err.(syscall.Errno))
								break
							}
						*/
					}
					/* set mode */
					if newattr.Mode.Set_it {
						if err := os.Chmod(fileName, os.FileMode(newattr.Mode.Mode)); err != nil {
							log.Printf("Setattr: chmod failed (err %v)", err)
							status = getSetattrErr(err.(*os.PathError).Err.(syscall.Errno))
							break
						}
					}
					/* set time */
					atime := fattr.Atime
					mtime := fattr.Mtime
					if newattr.Atime.Set_it != DONT_CHANGE {
						atime = Nfstime3(newattr.Atime.Atime)
					}
					if newattr.Mtime.Set_it != DONT_CHANGE {
						mtime = Nfstime3(newattr.Mtime.Mtime)
					}
					if newattr.Atime.Set_it != DONT_CHANGE || newattr.Mtime.Set_it != DONT_CHANGE {
						aTtime := time.Unix(int64(atime.Seconds), int64(atime.Nseconds))
						mTtime := time.Unix(int64(mtime.Seconds), int64(mtime.Nseconds))
						if err := os.Chtimes(fileName, aTtime, mTtime); err != nil {
							log.Printf("Setattr: chtimes failed (err %v)", err)
							status = getSetattrErr(err.(*os.PathError).Err.(syscall.Errno))
							break
						}
					}
					runOnce = false
					status = NFS3_OK
				}
				/* stat fileName for post attributes */
				if err = be.getFileAttrFromPath(fileName, &reply.Resok.Obj_wcc.After.Attributes); err == nil {
					reply.Resok.Obj_wcc.After.Attributes_follow = true
				} else {
					/* we just set attributes on this fileName ! */
					log.Printf("Setattr: poststat failed on fileName %s,deleting fHandle %d", fileName, fHandle)
					be.deleteHandle(fHandle)
				}
			} else {
				log.Printf("Setattr: guard.Check failed(%d %d)", guard.Obj_ctime.Seconds, preOpAttr.Ctime.Seconds)
				status = NFS3ERR_NOT_SYNC
				/* stat fileName for post attributes */
				if err = be.getFileAttrFromPath(fileName, &reply.Resfail.Obj_wcc.After.Attributes); err == nil {
					reply.Resfail.Obj_wcc.After.Attributes_follow = true
				} else {
					/* we just stated this fileName ! */
					log.Printf("Setattr: poststat failed on fileName %s,deleting fHandle %d", fileName, fHandle)
					be.deleteHandle(fHandle)
				}
			}
		} else {
			log.Printf("Setattr: stat failed on fileName %s,deleting fHandle %d", fileName, fHandle)
			be.deleteHandle(fHandle)
		}
	} else {
		log.Printf("Setattr: fHandle %d not found", fHandle)
	}
	return status
}

func getReadErr(sysError syscall.Errno) (status Nfsstat3) {
	switch {
	case sysError == syscall.EACCES:
		status = NFS3ERR_ACCES
	case sysError == syscall.ENAMETOOLONG || sysError == syscall.ELOOP ||
		sysError == syscall.ENOTDIR || sysError == syscall.ENOENT:
		status = NFS3ERR_STALE
	case sysError == syscall.ENXIO || sysError == syscall.ENODEV:
		status = NFS3ERR_NXIO
	case sysError == syscall.EINVAL:
		status = NFS3ERR_INVAL
	default:
		status = NFS3ERR_IO
	}
	return status
}

func (be *Backend) Read(fHandle uint64, offset uint64, count uint32, reply *READ3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	var fattr Fattr3
	if fileName, found := be.getValuefromHandle(fHandle); found {
		if err := be.getFileAttrFromPath(fileName, &fattr); err == nil {

			if fattr.Type == NF3REG {
				if fType, err := os.Open(fileName); err == nil {
					defer fType.Close()
					if count > tcpMaxData {
						count = tcpMaxData
					}
					reply.Resok.Data = make([]byte, count, count)
					if bytesRead, err := fType.ReadAt(reply.Resok.Data, int64(offset)); err == nil || err == io.EOF || bytesRead != 0 {
						/* from here we always return NFS3_OK, even if poststat fail */
						reply.Resok.Count = Count3(bytesRead)
						if err == io.EOF {
							reply.Resok.Eof = true
							if bytesRead == 0 {
								reply.Resok.Data = nil
							}
						} else {
							reply.Resok.Eof = false
						}
						/* stat fileName for post attributes */
						if err = be.getFileAttrFromPath(fileName, &reply.Resok.File_attributes.Attributes); err == nil {
							reply.Resok.File_attributes.Attributes_follow = true
						} else {
							/* we just read from  this file ! */
							log.Printf("Read: poststat failed on fileName %s,deleting fHandle %d", fileName, fHandle)
							be.deleteHandle(fHandle)
						}
						status = NFS3_OK
					} else {
						log.Printf("Read:read failed (err %v)", err)
						status = getReadErr(err.(*os.PathError).Err.(syscall.Errno))
						/* stat fileName for post attributes */
						if err = be.getFileAttrFromPath(fileName, &reply.Resfail.File_attributes.Attributes); err == nil {
							reply.Resfail.File_attributes.Attributes_follow = true
						} else {
							/* we just read from  this file ! */
							log.Printf("Read: poststat failed on fileName %s,deleting fHandle %d", fileName, fHandle)
							be.deleteHandle(fHandle)
						}
					}
				} else {
					log.Printf("Read: open failed (err %v)", err)
					status = getReadErr(err.(*os.PathError).Err.(syscall.Errno))
					/* stat fileName for post attributes */
					if err = be.getFileAttrFromPath(fileName, &reply.Resfail.File_attributes.Attributes); err == nil {
						reply.Resfail.File_attributes.Attributes_follow = true
					} else {
						/* we just stated this file ! */
						log.Printf("Read: poststat failed on fileName %s,deleting fHandle %d", fileName, fHandle)
						be.deleteHandle(fHandle)
					}
				}
			} else {
				log.Printf("Read: type check failed on fileName %s(not regular)", fileName)
				status = NFS3ERR_INVAL
				/* stat fileName for post attributes */
				if err = be.getFileAttrFromPath(fileName, &reply.Resfail.File_attributes.Attributes); err == nil {
					reply.Resfail.File_attributes.Attributes_follow = true
				} else {
					/* we just stated this file ! */
					log.Printf("Read: poststat failed on fileName %s,deleting fHandle %d", fileName, fHandle)
					be.deleteHandle(fHandle)
				}
			}
		} else {
			log.Printf("Read: stat failed on fileName %s,deleting fHandle %d", fileName, fHandle)
			be.deleteHandle(fHandle)
		}
	} else {
		log.Printf("Read: fHandle %d not found", fHandle)
	}
	return status
}

func getWriteErr(sysError syscall.Errno) (status Nfsstat3) {
	switch {
	case sysError == syscall.EACCES:
		status = NFS3ERR_ACCES
	case sysError == syscall.ENAMETOOLONG || sysError == syscall.ELOOP ||
		sysError == syscall.ENOTDIR || sysError == syscall.ENOENT:
		status = NFS3ERR_STALE
	case sysError == syscall.EROFS:
		status = NFS3ERR_ROFS
	case sysError == syscall.ENOSPC:
		status = NFS3ERR_NOSPC
	case sysError == syscall.EFBIG:
		status = NFS3ERR_FBIG
	case sysError == syscall.EINVAL:
		status = NFS3ERR_INVAL
	default:
		status = NFS3ERR_IO
	}
	return status
}
func (be *Backend) Write(fHandle uint64, offset uint64, count uint32, stable Stable_how, data []byte, reply *WRITE3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	var fattr Fattr3
	if fileName, found := be.getValuefromHandle(fHandle); found {
		if err := be.getFileAttrFromPath(fileName, &fattr); err == nil {
			preOpAttr := Wcc_attr{Size: fattr.Size, Ctime: fattr.Ctime, Mtime: fattr.Mtime}
			reply.Resok.File_wcc.Before.Attributes = preOpAttr
			reply.Resok.File_wcc.Before.Attributes_follow = true
			/* preset these values in case we fail */
			reply.Resfail.File_wcc.Before.Attributes = preOpAttr
			reply.Resfail.File_wcc.Before.Attributes_follow = true

			/* check if export is read-only */
			if be.config.writeaccess == false {
				status = NFS3ERR_ROFS
				reply.Resfail.File_wcc.After.Attributes = fattr
				reply.Resfail.File_wcc.After.Attributes_follow = true
				return status
			}

			if fattr.Type == NF3REG {
				if fType, err := os.OpenFile(fileName, os.O_WRONLY, 0666); err == nil {
					defer fType.Close()
					if count > tcpMaxData {
						count = tcpMaxData
					}
					if bytesWritten, err := fType.WriteAt(data, int64(offset)); err == nil || bytesWritten != 0 {

						reply.Resok.Count = Count3(bytesWritten)
						reply.Resok.Committed = UNSTABLE
						if stable != UNSTABLE { /* sync request from client */
							if err = fType.Sync(); err == nil {
								reply.Resok.Committed = FILE_SYNC
							} else {
								log.Printf("Write: sync failed, ignoring(err %v)", err)
							}
						}
						/* NFSWriteVerfier is used to allow a client to detect server state change
						 * Currently we loose all file handles when server crashes, but writing it anyway */
						binary.LittleEndian.PutUint64(reply.Resok.Verf[:8], uint64(be.ServerStartTime))

						/* stat fileName for post attributes */
						if err = be.getFileAttrFromPath(fileName, &reply.Resok.File_wcc.After.Attributes); err == nil {
							reply.Resok.File_wcc.After.Attributes_follow = true
						} else {
							/* we just wrote to this file ! */
							log.Printf("Write: poststat failed on fileName %s,deleting fHandle %d", fileName, fHandle)
							be.deleteHandle(fHandle)
						}
						status = NFS3_OK
					} else {
						log.Printf("Write: write failed (err %v)", err)
						status = getWriteErr(err.(*os.PathError).Err.(syscall.Errno))
						/* stat fileName for post attributes */
						if err = be.getFileAttrFromPath(fileName, &reply.Resfail.File_wcc.After.Attributes); err == nil {
							reply.Resfail.File_wcc.After.Attributes_follow = true
						} else {
							/* we just stated this file ! */
							log.Printf("Write: poststat failed on fileName %s,deleting fHandle %d", fileName, fHandle)
							be.deleteHandle(fHandle)
						}
					}
				} else {
					log.Printf("Write: open failed (err %v)", err)
					status = getWriteErr(err.(*os.PathError).Err.(syscall.Errno))
					/* stat fileName for post attributes */
					if err = be.getFileAttrFromPath(fileName, &reply.Resfail.File_wcc.After.Attributes); err == nil {
						reply.Resfail.File_wcc.After.Attributes_follow = true
					} else {
						/* we just stated this file ! */
						log.Printf("Write: poststat failed on fileName %s,deleting fHandle %d", fileName, fHandle)
						be.deleteHandle(fHandle)
					}
				}
			} else {
				log.Printf("Write: type check failed on fileName %s(not regular)", fileName)
				status = NFS3ERR_INVAL
				/* stat fileName for post attributes */
				if err = be.getFileAttrFromPath(fileName, &reply.Resfail.File_wcc.After.Attributes); err == nil {
					reply.Resfail.File_wcc.After.Attributes_follow = true
				} else {
					/* we just stated this file ! */
					log.Printf("Write: poststat failed on fileName %s,deleting fHandle %d", fileName, fHandle)
					be.deleteHandle(fHandle)
				}
			}
		} else {
			log.Printf("Write: stat failed on fileName %s,deleting fHandle %d", fileName, fHandle)
			be.deleteHandle(fHandle)
		}
	} else {
		log.Printf("Write: fHandle %d not found", fHandle)
	}
	return status
}

func (be *Backend) Commit(fHandle uint64, reply *COMMIT3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	var fattr Fattr3
	if fileName, found := be.getValuefromHandle(fHandle); found {
		if err := be.getFileAttrFromPath(fileName, &fattr); err == nil {
			preOpAttr := Wcc_attr{Size: fattr.Size, Ctime: fattr.Ctime, Mtime: fattr.Mtime}
			reply.Resok.File_wcc.Before.Attributes = preOpAttr
			reply.Resok.File_wcc.Before.Attributes_follow = true
			/* preset these values in case we fail */
			reply.Resfail.File_wcc.Before.Attributes = preOpAttr
			reply.Resfail.File_wcc.Before.Attributes_follow = true

			/* check if export is read-only */
			if be.config.writeaccess == false {
				status = NFS3ERR_ROFS
				reply.Resfail.File_wcc.After.Attributes = fattr
				reply.Resfail.File_wcc.After.Attributes_follow = true
				return status
			}
			if fattr.Type == NF3REG {
				/* currently, not doing explicit sync here, leave it to kernel.Send OK */
				reply.Resok.File_wcc.After.Attributes = fattr
				reply.Resok.File_wcc.After.Attributes_follow = true
				binary.LittleEndian.PutUint64(reply.Resok.Verf[:8], uint64(be.ServerStartTime))
				status = NFS3_OK
			} else {
				log.Printf("Commit: type check failed on fileName %s(not regular)", fileName)
				status = NFS3ERR_INVAL
				/* for now, No stat on fileName for post attributes. Send what we already have */
				reply.Resfail.File_wcc.After.Attributes = fattr
				reply.Resfail.File_wcc.After.Attributes_follow = true
			}
		} else {
			log.Printf("Commit: stat failed on fileName %s,deleting fHandle %d", fileName, fHandle)
			be.deleteHandle(fHandle)
		}
	} else {
		log.Printf("Commit: fHandle %d not found", fHandle)
	}
	return status
}

func getRemoveErr(sysError syscall.Errno) (status Nfsstat3) {
	switch {
	case sysError == syscall.EACCES || sysError == syscall.EPERM:
		status = NFS3ERR_ACCES
	case sysError == syscall.ENOENT:
		status = NFS3ERR_NOENT
	case sysError == syscall.ENAMETOOLONG || sysError == syscall.ELOOP ||
		sysError == syscall.ENOTDIR:
		status = NFS3ERR_STALE
	case sysError == syscall.EROFS:
		status = NFS3ERR_ROFS
	case sysError == syscall.EINVAL:
		status = NFS3ERR_INVAL
	default:
		status = NFS3ERR_IO
	}
	return status
}

func (be *Backend) Remove(dirFHandle uint64, name string, reply *REMOVE3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	var fattr Fattr3
	if dirName, found := be.getValuefromHandle(dirFHandle); found {
		if err := be.getFileAttrFromPath(dirName, &fattr); err == nil {
			preOpAttr := Wcc_attr{Size: fattr.Size, Ctime: fattr.Ctime, Mtime: fattr.Mtime}
			reply.Resok.Dir_wcc.Before.Attributes = Wcc_attr(preOpAttr)
			reply.Resok.Dir_wcc.Before.Attributes_follow = true
			/* set these now in case we fail later */
			reply.Resfail.Dir_wcc.Before.Attributes = preOpAttr
			reply.Resfail.Dir_wcc.Before.Attributes_follow = true

			/* check if export is read-only */
			if be.config.writeaccess == false {
				status = NFS3ERR_ROFS
				reply.Resfail.Dir_wcc.After.Attributes = fattr
				reply.Resfail.Dir_wcc.After.Attributes_follow = true
				return status
			}

			/* check for invalid names: dont allow dot and dotdot  */
			if name != "" && strings.ContainsAny(name, "/") == false && name != "." && name != ".." {
				/* Remove can unlink a file as well as do rmdir on a directory.
				   Misbehaving client could send a dir name instead of file name. use syscall */
				filename := dirName + "/" + name
				if err = syscall.Unlink(filename); err == nil {
					/* stat dirName for post attributes */
					if err = be.getFileAttrFromPath(dirName, &reply.Resok.Dir_wcc.After.Attributes); err == nil {
						reply.Resok.Dir_wcc.After.Attributes_follow = true
					} else {
						/* we just removed a file from this dir ! */
						log.Printf("Remove: poststat failed on dirName %s,deleting fHandle %d", dirName, dirFHandle)
						be.deleteHandle(dirFHandle)
					}
					status = NFS3_OK
				} else {
					log.Printf("Remove: Unlink failed (err %v)", err)
					status = getRemoveErr(err.(syscall.Errno))
					/* stat dirName for post attributes */
					if err = be.getFileAttrFromPath(dirName, &reply.Resfail.Dir_wcc.After.Attributes); err == nil {
						reply.Resfail.Dir_wcc.After.Attributes_follow = true
					} else {
						/* we just stated this dir ! */
						log.Printf("Remove: poststat failed on dirName %s,deleting fHandle %d", dirName, dirFHandle)
						be.deleteHandle(dirFHandle)
					}
				}
			} else {
				log.Printf("Remove: invalid name %s", name)
				status = NFS3ERR_INVAL
				/* stat dirName for post attributes */
				if err = be.getFileAttrFromPath(dirName, &reply.Resfail.Dir_wcc.After.Attributes); err == nil {
					reply.Resfail.Dir_wcc.After.Attributes_follow = true
				} else {
					/* we just stated this dir ! */
					log.Printf("Remove: poststat failed on dirName %s,deleting fHandle %d", dirName, dirFHandle)
					be.deleteHandle(dirFHandle)
				}
			}
		} else {
			log.Printf("Remove: stat failed on dirName %s,deleting fHandle %d", dirName, dirFHandle)
			be.deleteHandle(dirFHandle)
		}
	} else {
		log.Printf("Remove: fHandle %d not found", dirFHandle)
	}
	return status
}

func getRmdirErr(sysError syscall.Errno) (status Nfsstat3) {
	switch {
	case sysError == syscall.ENOTEMPTY:
		status = NFS3ERR_NOTEMPTY
	case sysError == syscall.EACCES || sysError == syscall.EPERM:
		status = NFS3ERR_ACCES
	case sysError == syscall.ENOENT:
		status = NFS3ERR_NOENT
	case sysError == syscall.ENAMETOOLONG || sysError == syscall.ELOOP ||
		sysError == syscall.ENOTDIR:
		status = NFS3ERR_STALE
	case sysError == syscall.EROFS:
		status = NFS3ERR_ROFS
	case sysError == syscall.EINVAL:
		status = NFS3ERR_INVAL
	default:
		status = NFS3ERR_IO
	}
	return status
}

func (be *Backend) Rmdir(dirFHandle uint64, name string, reply *RMDIR3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	var fattr Fattr3
	if dirName, found := be.getValuefromHandle(dirFHandle); found {
		if err := be.getFileAttrFromPath(dirName, &fattr); err == nil {
			preOpAttr := Wcc_attr{Size: fattr.Size, Ctime: fattr.Ctime, Mtime: fattr.Mtime}
			reply.Resok.Dir_wcc.Before.Attributes = preOpAttr
			reply.Resok.Dir_wcc.Before.Attributes_follow = true
			/* set these now in case we fail later */
			reply.Resfail.Dir_wcc.Before.Attributes = preOpAttr
			reply.Resfail.Dir_wcc.Before.Attributes_follow = true

			/* check if export is read-only */
			if be.config.writeaccess == false {
				status = NFS3ERR_ROFS
				reply.Resfail.Dir_wcc.After.Attributes = fattr
				reply.Resfail.Dir_wcc.After.Attributes_follow = true
				return status
			}

			/* check for invalid names: dont allow dot and dotdot  */
			if name != "" || strings.ContainsAny(name, "/") == false && name != "." && name != ".." {
				/* Remove can unlink a file as well as do rmdir on a directory.
				   Misbehaving client could send a file name instead of dir name. Use syscall */
				dirname := dirName + "/" + name
				if err = syscall.Rmdir(dirname); err == nil {
					/* stat dirName for post attributes */
					if err = be.getFileAttrFromPath(dirName, &reply.Resok.Dir_wcc.After.Attributes); err == nil {
						reply.Resok.Dir_wcc.After.Attributes_follow = true
					} else {
						/* we just removed a dir in this dir ! */
						log.Printf("Rmdir: poststat failed on dirName %s,deleting fHandle %d", dirName, dirFHandle)
						be.deleteHandle(dirFHandle)
					}
					status = NFS3_OK
				} else {
					log.Printf("Rmdir: Rmdir failed (err %v)", err)
					status = getRmdirErr(err.(syscall.Errno))
					/* stat dirName for post attributes */
					if err = be.getFileAttrFromPath(dirName, &reply.Resfail.Dir_wcc.After.Attributes); err == nil {
						reply.Resfail.Dir_wcc.After.Attributes_follow = true
					} else {
						/* we just stated this dir ! */
						log.Printf("Rmdir: poststat failed on dirName %s,deleting fHandle %d", dirName, dirFHandle)
						be.deleteHandle(dirFHandle)
					}
				}
			} else {
				log.Printf("Rmdir: invalid name %s", name)
				status = NFS3ERR_INVAL
				/* stat dirName for post attributes */
				if err = be.getFileAttrFromPath(dirName, &reply.Resfail.Dir_wcc.After.Attributes); err == nil {
					reply.Resfail.Dir_wcc.After.Attributes_follow = true
				} else {
					/* we just stated this dir ! */
					log.Printf("Rmdir: poststat failed on dirName %s,deleting fHandle %d", dirName, dirFHandle)
					be.deleteHandle(dirFHandle)
				}
			}
		} else {
			log.Printf("Rmdir: stat failed on dirName %s,deleting fHandle %d", dirName, dirFHandle)
			be.deleteHandle(dirFHandle)
		}
	} else {
		log.Printf("Rmdir: fHandle %d not found", dirFHandle)
	}
	return status
}

func getLinkErr(sysError syscall.Errno) (status Nfsstat3) {
	switch {
	case sysError == syscall.EXDEV:
		status = NFS3ERR_XDEV
	case sysError == syscall.EMLINK:
		status = NFS3ERR_MLINK
	case sysError == syscall.EACCES || sysError == syscall.EPERM:
		status = NFS3ERR_ACCES
	case sysError == syscall.ENAMETOOLONG || sysError == syscall.ELOOP ||
		sysError == syscall.ENOTDIR || sysError == syscall.ENOENT:
		status = NFS3ERR_STALE
	case sysError == syscall.EROFS:
		status = NFS3ERR_ROFS
	case sysError == syscall.EEXIST:
		status = NFS3ERR_EXIST
	case sysError == syscall.ENOSPC:
		status = NFS3ERR_NOSPC
	case sysError == syscall.ENOSYS:
		status = NFS3ERR_NOTSUPP
	case sysError == syscall.EINVAL:
		status = NFS3ERR_INVAL
	default:
		status = NFS3ERR_IO
	}
	return status
}
func (be *Backend) Link(oldFHandle uint64, linkDirFHandle uint64, newName string, reply *LINK3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	var fattr Fattr3
	if linkDir, found := be.getValuefromHandle(linkDirFHandle); found {
		if err := be.getFileAttrFromPath(linkDir, &fattr); err == nil {
			preOpAttr := Wcc_attr{Size: fattr.Size, Ctime: fattr.Ctime, Mtime: fattr.Mtime}
			reply.Resok.Linkdir_wcc.Before.Attributes_follow = true
			reply.Resok.Linkdir_wcc.Before.Attributes = preOpAttr
			/* preset these values in case we fail */
			reply.Resfail.Linkdir_wcc.Before.Attributes_follow = true
			reply.Resfail.Linkdir_wcc.Before.Attributes = preOpAttr

			if oldName, found := be.getValuefromHandle(oldFHandle); found {
				var oldFattr Fattr3
				if err = be.getFileAttrFromPath(oldName, &oldFattr); err == nil {
					/* check if export is read-only */
					if be.config.writeaccess == false {
						status = NFS3ERR_ROFS
						reply.Resfail.Linkdir_wcc.After.Attributes_follow = true
						reply.Resfail.Linkdir_wcc.After.Attributes = fattr
						reply.Resfail.File_attributes.Attributes = oldFattr
						reply.Resfail.File_attributes.Attributes_follow = true
						return status
					}
					/* check for invalid names: dont allow dot and dotdot  */
					if newName != "" && strings.ContainsAny(newName, "/") == false && newName != "." && newName != ".." {
						if err = os.Link(oldName, linkDir+"/"+newName); err == nil {

							/* stat linkDir for post attributes */
							if err = be.getFileAttrFromPath(linkDir, &reply.Resok.Linkdir_wcc.After.Attributes); err == nil {
								reply.Resok.Linkdir_wcc.After.Attributes_follow = true
								/* stat oldName for post attributes */
								if err = be.getFileAttrFromPath(oldName, &reply.Resok.File_attributes.Attributes); err == nil {
									reply.Resok.File_attributes.Attributes_follow = true
								} else {
									/* we just created a link of this file in linkDir! */
									log.Printf("Link: failed poststat on oldName %s,deleting oldFHandle %d", oldName, oldFHandle)
									reply.Resok.File_attributes.Attributes_follow = false
									be.deleteHandle(oldFHandle)
								}
							} else {
								/* we just created a link of oldName in this dir! */
								log.Printf("Link: failed poststat on dirName %s,deleting linkDirFHandle %d", linkDir, linkDirFHandle)
								reply.Resok.Linkdir_wcc.After.Attributes_follow = false
								be.deleteHandle(linkDirFHandle)
							}
							status = NFS3_OK
						} else {
							log.Printf("Link: link failed (err %v)", err)
							status = getLinkErr(err.(*os.LinkError).Err.(syscall.Errno))
							/* stat linkDir for post attributes */
							if err = be.getFileAttrFromPath(linkDir, &reply.Resfail.Linkdir_wcc.After.Attributes); err == nil {
								reply.Resfail.Linkdir_wcc.After.Attributes_follow = true
								/* stat oldName for post attributes */
								if err = be.getFileAttrFromPath(oldName, &reply.Resfail.File_attributes.Attributes); err == nil {
									reply.Resfail.File_attributes.Attributes_follow = true
								} else {
									/* we just stated oldName! */
									log.Printf("Link: failed poststat on oldName %s,deleting oldFHandle %d", oldName, oldFHandle)
									reply.Resfail.File_attributes.Attributes_follow = false
									be.deleteHandle(oldFHandle)
								}
							} else {
								/* we just stated linkDir! */
								log.Printf("Link: failed poststat on dirName %s,deleting linkDirFHandle %d", linkDir, linkDirFHandle)
								reply.Resfail.Linkdir_wcc.After.Attributes_follow = false
								be.deleteHandle(linkDirFHandle)
							}
						}
					} else {
						log.Printf("Link: invalid newName %s", newName)
						status = NFS3ERR_INVAL
						/* stat linkDir for post attributes */
						if err = be.getFileAttrFromPath(linkDir, &reply.Resfail.Linkdir_wcc.After.Attributes); err == nil {
							reply.Resfail.Linkdir_wcc.After.Attributes_follow = true
							/* stat oldName for post attributes */
							if err = be.getFileAttrFromPath(oldName, &reply.Resfail.File_attributes.Attributes); err == nil {
								reply.Resfail.File_attributes.Attributes_follow = true
							} else {
								/* we just stated oldName! */
								log.Printf("Link: failed poststat on oldName %s,deleting oldFHandle %d", oldName, oldFHandle)
								reply.Resfail.File_attributes.Attributes_follow = false
								be.deleteHandle(oldFHandle)
							}
						} else {
							/* we just stated linkDir! */
							log.Printf("Link: failed poststat on dirName %s,deleting linkDirFHandle %d", linkDir, linkDirFHandle)
							reply.Resfail.Linkdir_wcc.After.Attributes_follow = false
							be.deleteHandle(linkDirFHandle)
						}
					}
				} else {
					log.Printf("Link: stat failed on oldName %s", oldName)
					be.deleteHandle(oldFHandle)
					/* stat linkDir for post attributes */
					if err = be.getFileAttrFromPath(linkDir, &reply.Resfail.Linkdir_wcc.After.Attributes); err == nil {
						reply.Resfail.Linkdir_wcc.After.Attributes_follow = true
					} else {
						/* we just stated linkDir! */
						log.Printf("Link: failed poststat on dirName %s,deleting linkDirFHandle %d", linkDir, linkDirFHandle)
						reply.Resfail.Linkdir_wcc.After.Attributes_follow = false
						be.deleteHandle(linkDirFHandle)
					}
				}
			} else {
				log.Printf("Link: oldFHandle %d not found", oldFHandle)
				/* stat linkDir for post attributes */
				if err = be.getFileAttrFromPath(linkDir, &reply.Resfail.Linkdir_wcc.After.Attributes); err == nil {
					reply.Resfail.Linkdir_wcc.After.Attributes_follow = true
				} else {
					/* we just stated linkDir! */
					log.Printf("Link: failed poststat on dirName %s,deleting linkDirFHandle %d", linkDir, linkDirFHandle)
					reply.Resfail.Linkdir_wcc.After.Attributes_follow = false
					be.deleteHandle(linkDirFHandle)
				}
			}

		} else {
			log.Printf("Link: stat failed on linkDir %s,deleting linkDirFHandle %d", linkDir, linkDirFHandle)
			be.deleteHandle(linkDirFHandle)
			/* check if oldName exists for post failure stats */
			if oldName, found := be.getValuefromHandle(oldFHandle); found {
				if err := be.getFileAttrFromPath(oldName, &reply.Resfail.File_attributes.Attributes); err == nil {
					reply.Resfail.File_attributes.Attributes_follow = true
				} else {
					log.Printf("Link: poststat failed on oldName %s,deleting oldFHandle %d", oldName, oldFHandle)
					be.deleteHandle(oldFHandle)
				}
			} else {
				log.Printf("Link: oldFHandle %d not found", oldFHandle)
			}
		}
	} else {
		log.Printf("Link: linkDirFHandle %d not found", linkDirFHandle)
		/* check if oldName exists for post failure stats */
		if oldName, found := be.getValuefromHandle(oldFHandle); found {
			if err := be.getFileAttrFromPath(oldName, &reply.Resfail.File_attributes.Attributes); err == nil {
				reply.Resfail.File_attributes.Attributes_follow = true
			} else {
				log.Printf("Link: poststat failed on oldName %s,deleting oldFHandle %d", oldName, oldFHandle)
				be.deleteHandle(oldFHandle)
			}
		} else {
			log.Printf("Link: oldFHandle %d not found", oldFHandle)
		}
	}
	return status
}

func getSymLinkErr(sysError syscall.Errno) (status Nfsstat3) {
	switch {
	case sysError == syscall.EACCES || sysError == syscall.EPERM:
		status = NFS3ERR_ACCES
	case sysError == syscall.ENAMETOOLONG || sysError == syscall.ELOOP ||
		sysError == syscall.ENOTDIR || sysError == syscall.ENOENT:
		status = NFS3ERR_STALE
	case sysError == syscall.EROFS:
		status = NFS3ERR_ROFS
	case sysError == syscall.EEXIST:
		status = NFS3ERR_EXIST
	case sysError == syscall.ENOSPC:
		status = NFS3ERR_NOSPC
	case sysError == syscall.ENOSYS:
		status = NFS3ERR_NOTSUPP
	case sysError == syscall.EINVAL:
		status = NFS3ERR_INVAL
	default:
		status = NFS3ERR_IO
	}
	return status
}
func (be *Backend) SymLink(linkDirFHandle uint64, symLinkName string, symlinkData string,
	newattr Sattr3, reply *SYMLINK3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	var fattr Fattr3
	if linkDir, found := be.getValuefromHandle(linkDirFHandle); found {
		if err := be.getFileAttrFromPath(linkDir, &fattr); err == nil {
			preOpAttr := Wcc_attr{Size: fattr.Size, Ctime: fattr.Ctime, Mtime: fattr.Mtime}
			reply.Resok.Dir_wcc.Before.Attributes = preOpAttr
			reply.Resok.Dir_wcc.Before.Attributes_follow = true
			/* preset these values in case we fail */
			reply.Resfail.Dir_wcc.Before.Attributes = preOpAttr
			reply.Resfail.Dir_wcc.Before.Attributes_follow = true

			/* check if export is read-only */
			if be.config.writeaccess == false {
				status = NFS3ERR_ROFS
				reply.Resfail.Dir_wcc.After.Attributes = fattr
				reply.Resfail.Dir_wcc.After.Attributes_follow = true
				return status
			}

			/* check for invalid names: dont allow dot and dotdot  */
			if symLinkName != "" && strings.ContainsAny(symLinkName, "/") == false && symLinkName != "." && symLinkName != ".." {
				newFilename := linkDir + "/" + symLinkName
				/* ignoring the open mode in newattr, use the default */
				if err = os.Symlink(symlinkData, newFilename); err == nil {
					if err = be.getFileAttrFromPath(newFilename, &reply.Resok.Obj_attributes.Attributes); err == nil {
						reply.Resok.Obj_attributes.Attributes_follow = true
						newFHandle := be.makeFHandle(newFilename, uint64(reply.Resok.Obj_attributes.Attributes.Fileid),
							reply.Resok.Obj_attributes.Attributes.Fsid)
						if foundname, found := be.getValuefromHandle(newFHandle); !found {
							be.setValueforHandle(newFHandle, newFilename)
						} else {
							/* if foundname no longer exists on filesystem, reset the handle with newFilename */
							if foundname != newFilename {
								if _, err := os.Lstat(foundname); err != nil {
									log.Printf("SymLink: resetting staleFHandle(foundname %s, newFilename %s)", foundname, newFilename)
									be.setValueforHandle(newFHandle, newFilename)
								} else {
									log.Fatalf("SymLink: FNV1a collision(foundname %s newFilename %s)", foundname, newFilename)
								}
							}
						}
						reply.Resok.Obj.Handle.Data = make([]byte, 8, 8)
						binary.LittleEndian.PutUint64(reply.Resok.Obj.Handle.Data, newFHandle)
						reply.Resok.Obj.Handle_follows = true
						/* stat linkDir for post attributes */
						if err = be.getFileAttrFromPath(linkDir, &reply.Resok.Dir_wcc.After.Attributes); err == nil {
							reply.Resok.Dir_wcc.After.Attributes_follow = true
						} else {
							/* we just created a symlink to a file in this dir ! */
							log.Printf("SymLink: failed poststat on linkDir %s,deleting linkDirFHandle %d", linkDir, linkDirFHandle)
							reply.Resok.Dir_wcc.After.Attributes_follow = false
							be.deleteHandle(linkDirFHandle)
						}
						status = NFS3_OK
					} else {
						/* we just created this symlink file ! */
						log.Printf("SymLink: failed stat on newFilename %s", newFilename)
						/* stat linkDir for post attributes */
						if err = be.getFileAttrFromPath(linkDir, &reply.Resfail.Dir_wcc.After.Attributes); err == nil {
							reply.Resfail.Dir_wcc.After.Attributes_follow = true
						} else {
							/* we just stated linkDir! */
							log.Printf("SymLink: failed poststat on linkDir %s,deleting linkDirFHandle %d", linkDir, linkDirFHandle)
							reply.Resfail.Dir_wcc.After.Attributes_follow = false
							be.deleteHandle(linkDirFHandle)
						}
					}
				} else {
					log.Printf("SymLink: failed (err %v)", err)
					status = getSymLinkErr(err.(*os.LinkError).Err.(syscall.Errno))
					/* stat linkDir for post attributes */
					if err = be.getFileAttrFromPath(linkDir, &reply.Resfail.Dir_wcc.After.Attributes); err == nil {
						reply.Resfail.Dir_wcc.After.Attributes_follow = true
					} else {
						/* we just stated linkDir! */
						log.Printf("SymLink: failed poststat on linkDir %s,deleting linkDirFHandle %d", linkDir, linkDirFHandle)
						reply.Resfail.Dir_wcc.After.Attributes_follow = false
						be.deleteHandle(linkDirFHandle)
					}
				}
			} else {
				log.Printf("SymLink: invalid symLinkName %s", symLinkName)
				status = NFS3ERR_INVAL
				/* stat linkDir for post attributes */
				if err = be.getFileAttrFromPath(linkDir, &reply.Resfail.Dir_wcc.After.Attributes); err == nil {
					reply.Resfail.Dir_wcc.After.Attributes_follow = true
				} else {
					/* we just stated linkDir! */
					log.Printf("SymLink: failed poststat on linkDir %s,deleting linkDirFHandle %d", linkDir, linkDirFHandle)
					reply.Resfail.Dir_wcc.After.Attributes_follow = false
					be.deleteHandle(linkDirFHandle)
				}
			}
		} else {
			log.Printf("SymLink: stat failed on linkDir %s,deleting linkDirFHandle %d", linkDir, linkDirFHandle)
			be.deleteHandle(linkDirFHandle)
		}
	} else {
		log.Printf("SymLink: linkDirFHandle %d not found", linkDirFHandle)
	}
	return status
}

func getReadLinkErr(sysError syscall.Errno) (status Nfsstat3) {
	switch {
	case sysError == syscall.EINVAL:
		status = NFS3ERR_INVAL
	case sysError == syscall.EACCES:
		status = NFS3ERR_ACCES
	case sysError == syscall.ENOSYS:
		status = NFS3ERR_NOTSUPP
	case sysError == syscall.ENAMETOOLONG || sysError == syscall.ELOOP ||
		sysError == syscall.ENOTDIR || sysError == syscall.ENOENT:
		status = NFS3ERR_STALE
	default:
		status = NFS3ERR_IO
	}
	return status
}

func (be *Backend) ReadLink(linkFHandle uint64, reply *READLINK3res) (status Nfsstat3) {
	status = NFS3ERR_STALE
	var fattr Fattr3
	if linkFile, found := be.getValuefromHandle(linkFHandle); found {
		if err := be.getFileAttrFromPath(linkFile, &fattr); err == nil {
			if symLinkData, err := os.Readlink(linkFile); err == nil {
				reply.Resok.Data = Nfspath3(symLinkData)
				/* stat linkFile for post attributes */
				if err = be.getFileAttrFromPath(linkFile, &reply.Resok.Symlink_attributes.Attributes); err == nil {
					reply.Resok.Symlink_attributes.Attributes_follow = true
				} else {
					/* we just stated linkFile! */
					log.Printf("ReadLink: failed poststat on linkFile %s,deleting linkFHandle %d", linkFile, linkFHandle)
					reply.Resok.Symlink_attributes.Attributes_follow = false
					be.deleteHandle(linkFHandle)
				}
				status = NFS3_OK
			} else {

				log.Printf("ReadLink: readlink failed (err %v)", err)
				status = getReadLinkErr(err.(*os.PathError).Err.(syscall.Errno))
				/* stat linkFile for post attributes */
				if err = be.getFileAttrFromPath(linkFile, &reply.Resfail.Symlink_attributes.Attributes); err == nil {
					reply.Resfail.Symlink_attributes.Attributes_follow = true
				} else {
					/* we just stated linkFile! */
					log.Printf("ReadLink: failed poststat on linkFile %s,deleting linkFHandle %d", linkFile, linkFHandle)
					reply.Resfail.Symlink_attributes.Attributes_follow = false
					be.deleteHandle(linkFHandle)
				}
			}
		} else {
			log.Printf("ReadLink: stat failed on linkFile %s,deleting linkFHandle %d", linkFile, linkFHandle)
			be.deleteHandle(linkFHandle)
		}
	} else {
		log.Printf("ReadLink: linkFHandle %d not found", linkFHandle)
	}
	return status
}
