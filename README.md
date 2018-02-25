
GONFS is a NFS version 3 filesystem written in golang as per RFC 1813 protocol specification.  
The server consists of 4 modules : NFS3 FileSystem Backend, Portmapper client,   
External Data Representation  and ONC RemoteProcedureCall

# Typical Usage
**Start gonfs on a Linux server**  
Build gonfs binary by running cmd 'go build' in top src directory.  
Run *rpcinfo -p* to start rpcbind service.  
Run gonfs as the default user.  
Example : *./gonfs -allowAddr 192.168.109.0/24 -exportPath /home/noroot/storagXpert/mountDir*  
Other Configuration options : .*/gonfs -h*  

**Access from a Linux Client**  
Example : *sudo mount -t nfs -o nfsvers=3,tcp,nolock,noacl,intr,soft,rw  
           192.168.109.101:/home/noroot/storagXpert/mountDir/home/noroot/localMnt*  
(gonfs server is running on 192.168.109.101)  

**Access from Windows10 Client**  
Goto *Control Panel->Programs>Programs and Features->Turn Windows Feature on or off*  
Turn on Windows10  'Client for NFS' under 'Services for NFS' feature.  
Run *regedit* and goto *HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\ClientForNFS\CurrentVersion\Default*  
Create  a new DWORD(32-bit) Value 'AnonymousUid' and assign the UID shown by the gonfs server start message.  
Create  a new DWORD(32-bit) Value 'AnonymousGid ' and assign the GID shown by the gonfs server start message.  

Now, you can  mount NFS exported directory by gonfs from windows cmd prompt.  
Example : *mount -o anon -o nolock \\192.168.109.101\home\noroot\storagXpert\mountDir Y:*  
(gonfs server is running on 192.168.109.101)  

# Testing
GONFS Passes **Connectathon NFS tests**.  
There are 4 types of test : basic, general, special and lock.  
gonfs does not support NLM protocol.  

**Steps to run test suite**  
Start gonfs on a linux server.  
On a Linux Client,  
*git clone https://github.com/phdeniel/cthon04.git*  
*sudo make all*  
Now, you can run basic, general and special tests.  
Example :  
*./server -b -o "nfsvers=3,tcp,nolock,noacl,intr,soft,lookupcache=none,rw" -p "/home/noroot/storagXpert/mountDir" -m "/home/noroot/localMnt"  192.168.109.101  
./server -g -o "nfsvers=3,tcp,nolock,noacl,intr,soft,lookupcache=none,rw" -p "/home/noroot/storagXpert/mountDir" -m "/home/noroot/localMnt"  192.168.109.101  
./server -s -o "nfsvers=3,tcp,nolock,noacl,intr,soft,lookupcache=none,rw" -p "/home/noroot/storagXpert/mountDir" -m "/home/noroot/localMnt"  192.168.109.101*  
(gonfs server is running on 192.168.109.101)  

# Development Status
1. No exports file support for elaborate nfs configuration.  
2. All access would be squashed to user running gonfs server.  
3. Server would return ESTALE for old fileHandles after a crash/reboot.  
4. No daemonization of gonfs server.  
5. No ACL, NLM support.  
6. Supported network is TCP only.  
7. gonfs currently runs on Linux/Unix only.  

# Bug Report and Help
e-mail : storagxpert@gmail.com  
