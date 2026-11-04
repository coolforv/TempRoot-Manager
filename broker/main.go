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
 "time"
)

type Request struct {
 Op string `json:"op"`
 UID int `json:"uid,omitempty"`
 Args []string `json:"args,omitempty"`
}
type Reply struct {
 OK bool `json:"ok"`
 Error string `json:"error,omitempty"`
 EUID int `json:"euid,omitempty"`
 Grants []int `json:"grants,omitempty"`
 Pending []int `json:"pending,omitempty"`
}
type Broker struct {
 grants string
 managerUID int
 mu sync.RWMutex
 allowed map[int]bool
 pending map[int]time.Time
}

func peerUID(c *net.UnixConn) (int,error) {
 raw,e:=c.SyscallConn();if e!=nil{return -1,e}
 uid:=-1;var sockerr error
 e=raw.Control(func(fd uintptr) {
  cr,err:=syscall.GetsockoptUcred(int(fd),syscall.SOL_SOCKET,syscall.SO_PEERCRED)
  if err!=nil{sockerr=err}else{uid=int(cr.Uid)}
 })
 if e!=nil{return -1,e}
 if sockerr!=nil{return -1,sockerr}
 return uid,nil
}
func(b *Broker)load()error{
 b.allowed=map[int]bool{};b.pending=map[int]time.Time{}
 raw,e:=os.ReadFile(b.grants)
 if errors.Is(e,os.ErrNotExist){return nil}
 if e!=nil{return e}
 for _,line:=range strings.Split(string(raw),"\n"){
  id,err:=strconv.Atoi(strings.TrimSpace(line))
  if err==nil&&id>=10000&&id!=b.managerUID{b.allowed[id]=true}
 }
 return nil
}
func(b *Broker)persistLocked()error{
 list:=make([]int,0,len(b.allowed))
 for id:=range b.allowed{list=append(list,id)}
 sort.Ints(list)
 rows:=make([]string,len(list))
 for i,id:=range list{rows[i]=strconv.Itoa(id)}
 if e:=os.MkdirAll(filepath.Dir(b.grants),0700);e!=nil{return e}
 tmp:=b.grants+".new"
 if e:=os.WriteFile(tmp,[]byte(strings.Join(rows,"\n")+"\n"),0600);e!=nil{return e}
 return os.Rename(tmp,b.grants)
}
func(b *Broker)handle(conn *net.UnixConn){
 defer conn.Close()
 _=conn.SetDeadline(time.Now().Add(15*time.Second))
 caller,e:=peerUID(conn)
 if e!=nil{_ = json.NewEncoder(conn).Encode(Reply{Error:"peer credentials unavailable"});return}
 reader:=bufio.NewReaderSize(conn,4096)
 line,e:=reader.ReadBytes('\n')
 if e!=nil||len(line)>4096{_ = json.NewEncoder(conn).Encode(Reply{Error:"malformed request"});return}
 var r Request
 if json.Unmarshal(line,&r)!=nil{_ = json.NewEncoder(conn).Encode(Reply{Error:"malformed JSON"});return}
 managed:=caller==b.managerUID
 switch r.Op{
 case "status","list":
  if !managed{_ = json.NewEncoder(conn).Encode(Reply{Error:"manager-only"});return}
  b.mu.RLock()
  grants:=make([]int,0,len(b.allowed));pending:=make([]int,0,len(b.pending))
  for id:=range b.allowed{grants=append(grants,id)}
  for id:=range b.pending{pending=append(pending,id)}
  b.mu.RUnlock();sort.Ints(grants);sort.Ints(pending)
  _ = json.NewEncoder(conn).Encode(Reply{OK:true,EUID:os.Geteuid(),Grants:grants,Pending:pending})
 case "grant","revoke","deny":
  if !managed{_ = json.NewEncoder(conn).Encode(Reply{Error:"manager-only"});return}
  if r.UID<10000||r.UID==b.managerUID{_ = json.NewEncoder(conn).Encode(Reply{Error:"invalid target UID"});return}
  b.mu.Lock()
  var err error
  switch r.Op{
  case "grant":b.allowed[r.UID]=true;delete(b.pending,r.UID);err=b.persistLocked()
  case "revoke":delete(b.allowed,r.UID);delete(b.pending,r.UID);err=b.persistLocked()
  case "deny":delete(b.pending,r.UID)
  }
  b.mu.Unlock()
  if err!=nil{_ = json.NewEncoder(conn).Encode(Reply{Error:err.Error()});return}
  _ = json.NewEncoder(conn).Encode(Reply{OK:true})
 case "shell":
  // No implicit grants: the first request is recorded for explicit user approval.
  b.mu.Lock();allowed:=b.allowed[caller]
  if caller<10000||caller==b.managerUID{allowed=false}
  if !allowed&&caller>=10000&&caller!=b.managerUID{b.pending[caller]=time.Now()}
  b.mu.Unlock()
  if !allowed{binaryReply(conn,126,[]byte("TempRoot: authorization required; open TempRoot Manager\n"));return}
  if len(r.Args)!=2||r.Args[0]!="-c"{binaryReply(conn,2,[]byte("TempRoot: only -c command supported\n"));return}
  cmd:=exec.Command("/system/bin/sh","-c",r.Args[1])
  // No long-lived interactive root shell in V0.2.
  cmd.Stdin=strings.NewReader("")
  var output bytes.Buffer
  cmd.Stdout=&output;cmd.Stderr=&output
  e:=cmd.Run();code:=0
  if e!=nil{var exitErr *exec.ExitError;if errors.As(e,&exitErr){code=exitErr.ExitCode()}else{code=126}}
  b.mu.RLock();stillAllowed:=b.allowed[caller];b.mu.RUnlock()
  if !stillAllowed{binaryReply(conn,126,[]byte("authorization revoked\n"));return}
  raw:=output.Bytes();if len(raw)>8*1024*1024{raw=raw[:8*1024*1024]}
  binaryReply(conn,code,raw)
 default:_ = json.NewEncoder(conn).Encode(Reply{Error:"unknown operation"})
 }
}
func binaryReply(c net.Conn,code int,buf []byte){
 var h[8]byte
 binary.LittleEndian.PutUint32(h[:4],uint32(len(buf)))
 binary.LittleEndian.PutUint32(h[4:],uint32(int32(code)))
 _,_=c.Write(h[:]);_,_=c.Write(buf)
}
func main(){
 socket:=flag.String("socket","/data/local/tmp/temproot.sock","Unix socket")
 grants:=flag.String("grants","/data/local/tmp/temproot/grants","private grant database")
 managerUID:=flag.Int("manager-uid",-1,"trusted manager application UID")
 flag.Parse()
 if os.Getuid()!=0||os.Geteuid()!=0{fmt.Fprintln(os.Stderr,"requires existing UID 0 process");os.Exit(1)}
 if *managerUID<10000{fmt.Fprintln(os.Stderr,"invalid manager UID");os.Exit(2)}
 b:=&Broker{managerUID:*managerUID,grants:*grants}
 if e:=b.load();e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)}
 _=os.Remove(*socket)
 ln,e:=net.ListenUnix("unix",&net.UnixAddr{Name:*socket,Net:"unix"})
 if e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)}
 defer func(){ln.Close();_=os.Remove(*socket)}()
 if e=os.Chmod(*socket,0666);e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)}
 fmt.Println("TempRoot Broker V0.2 active, manager UID",*managerUID)
 for{
  c,err:=ln.AcceptUnix()
  if err!=nil{continue}
  go b.handle(c)
 }
}
