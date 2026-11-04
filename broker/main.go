package main

import (
 "bufio"
 "bytes"
 "encoding/binary"
 "encoding/json"
 "errors"
 "flag"
 "fmt"
 "net"
 "os"
 "os/exec"
 "path/filepath"
 "sort"
 "strconv"
 "strings"
 "sync"
 "syscall"
)
type Request struct { Op string `json:"op"`; UID int `json:"uid,omitempty"`; Args []string `json:"args,omitempty"` }
type Broker struct { socket, grants string; managerUID int; mu sync.RWMutex; saveMu sync.Mutex; allowed map[int]bool }
func peerUID(c *net.UnixConn)(int,error){
 raw,e:=c.SyscallConn();if e!=nil{return -1,e};uid:=-1;var inner error
 e=raw.Control(func(fd uintptr){u,x:=syscall.GetsockoptUcred(int(fd),syscall.SOL_SOCKET,syscall.SO_PEERCRED);if x!=nil{inner=x}else{uid=int(u.Uid)}})
 if e!=nil{return -1,e};if inner!=nil{return -1,inner};return uid,nil
}
func(b *Broker)load()error{
 b.mu.Lock();defer b.mu.Unlock();b.allowed=map[int]bool{}
 data,e:=os.ReadFile(b.grants);if errors.Is(e,os.ErrNotExist){return nil};if e!=nil{return e}
 for _,line:=range strings.Split(string(data),"\n"){n,x:=strconv.Atoi(strings.TrimSpace(line));if x==nil&&n>0&&n!=b.managerUID{b.allowed[n]=true}}
 return nil
}
func(b *Broker)save()error{
 b.saveMu.Lock();defer b.saveMu.Unlock();b.mu.RLock();ids:=make([]int,0,len(b.allowed));for id:=range b.allowed{ids=append(ids,id)};b.mu.RUnlock();sort.Ints(ids)
 rows:=make([]string,len(ids));for i,id:=range ids{rows[i]=strconv.Itoa(id)}
 if e:=os.MkdirAll(filepath.Dir(b.grants),0700);e!=nil{return e};tmp:=b.grants+".new"
 if e:=os.WriteFile(tmp,[]byte(strings.Join(rows,"\n")+"\n"),0600);e!=nil{return e};if e:=os.Chmod(tmp,0600);e!=nil{return e};return os.Rename(tmp,b.grants)
}
func reply(c net.Conn,v any){_ = json.NewEncoder(c).Encode(v)}
func(b *Broker)handle(c *net.UnixConn){
 defer c.Close();uid,e:=peerUID(c);if e!=nil{reply(c,map[string]any{"ok":false,"error":"peer credential verification failed"});return}
 line,e:=bufio.NewReaderSize(c,4096).ReadString('\n');if e!=nil||len(line)>4096{reply(c,map[string]any{"ok":false,"error":"invalid request"});return}
 var r Request;if json.Unmarshal([]byte(line),&r)!=nil{reply(c,map[string]any{"ok":false,"error":"invalid JSON"});return}
 switch r.Op {
 case "status":
  if uid!=b.managerUID{reply(c,map[string]any{"ok":false,"error":"manager UID required"});return};b.mu.RLock();n:=len(b.allowed);b.mu.RUnlock();reply(c,map[string]any{"ok":true,"euid":os.Geteuid(),"grants":n})
 case "grant","revoke":
  if uid!=b.managerUID{reply(c,map[string]any{"ok":false,"error":"manager UID required"});return};if r.UID<=0||r.UID==b.managerUID{reply(c,map[string]any{"ok":false,"error":"unsafe UID"});return}
  b.mu.Lock();if r.Op=="grant"{b.allowed[r.UID]=true}else{delete(b.allowed,r.UID)};b.mu.Unlock();if e:=b.save();e!=nil{reply(c,map[string]any{"ok":false,"error":e.Error()});return};reply(c,map[string]any{"ok":true})
 case "shell":
  b.mu.RLock();ok:=b.allowed[uid];b.mu.RUnlock();if !ok{reply(c,map[string]any{"ok":false,"error":"UID not authorized"});return}
  if len(r.Args)!=2||r.Args[0]!="-c"{reply(c,map[string]any{"ok":false,"error":"only -c command mode supported"});return}
  cmd:=exec.Command("/system/bin/sh","-c",r.Args[1]);var out bytes.Buffer;cmd.Stdout=&out;cmd.Stderr=&out;cmd.Stdin=strings.NewReader("");err:=cmd.Run();code:=0
  if err!=nil{var ee *exec.ExitError;if errors.As(err,&ee){code=ee.ExitCode()}else{code=126}}
  var h [8]byte;binary.LittleEndian.PutUint32(h[:4],uint32(out.Len()));binary.LittleEndian.PutUint32(h[4:],uint32(int32(code)));_,_=c.Write(h[:]);_,_=c.Write(out.Bytes())
 default:reply(c,map[string]any{"ok":false,"error":"unknown operation"})
 }
}
func main(){
 sock:=flag.String("socket","/data/local/tmp/temproot.sock","Unix socket");grants:=flag.String("grants","/data/local/tmp/temproot.grants","grant database");manager:=flag.Int("manager-uid",-1,"trusted manager APK UID");flag.Parse()
 if os.Getuid()!=0||os.Geteuid()!=0{fmt.Fprintln(os.Stderr,"broker must be started by existing UID-0 worker");os.Exit(1)};if *manager<=0{fmt.Fprintln(os.Stderr,"invalid manager UID");os.Exit(2)}
 b:=&Broker{socket:*sock,grants:*grants,managerUID:*manager};if e:=b.load();e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)}
 _=os.Remove(*sock);ln,e:=net.ListenUnix("unix",&net.UnixAddr{Name:*sock,Net:"unix"});if e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)};defer func(){ln.Close();_=os.Remove(*sock)}()
 if e=os.Chmod(*sock,0666);e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)};fmt.Printf("broker ready; manager UID=%d\n",*manager)
 for{c,e:=ln.AcceptUnix();if e!=nil{continue};go b.handle(c)}
}
