package main

import (
   "fmt"
   "io/ioutil"
   "encoding/json"
   "os"
   "qlib"
   "sync"
)


func main() {
   aDb, err := NewUserDb("./userdb")
   if err != nil { panic(err) }
   aDb.user["u111111"] = &tUser{Nodes: map[string]int{"111111":1}}
   aDb.user["u222222"] = &tUser{Nodes: map[string]int{"222222":1}}
   aDb.user["u333333"] = &tUser{Nodes: map[string]int{"333333":1}}
   aDb.group["g1"] = &tGroup{Uid: map[string]tMember{
      "u111111": tMember{Alias: "111"},
      "u222222": tMember{Alias: "222"},
      "u333333": tMember{Alias: "333"},
   }}

   qlib.UDb = aDb
   qlib.Init("qstore")

   fmt.Printf("Starting Test Pass\n")
   qlib.InitTestClient(2)
   for a := 0; true; a++ {
      aDawdle := a == 1
      qlib.NewLink(qlib.NewTestClient(aDawdle))
   }
}

/* moved to qlib/testclient.go
const ( _=iota; eRegister; eAddNode; eLogin; eListEdit; ePost; ePing; eAck )

var sTestClientId chan int

type tTestClient struct {
   id, to int // who i am, who i send to
   count int // msg number
   deferLogin bool // test login timeout feature
   ack chan int // writer tells reader to issue ack to qlib
   closed bool // when about to shut down
   readDeadline time.Time // set by qlib
}

func InitTestClient(i int) {
   sTestClientId = make(chan int, i)
   for a:=1; a <= i; a++ {
      sTestClientId <- 111111 * a
   }
}

func NewTestClient(iDawdle bool) *tTestClient {
   a := <-sTestClientId
   return &tTestClient{
      id: a, to: a+111111,
      deferLogin: iDawdle,
      ack: make(chan int,10),
   }
}

func (o *tTestClient) Read(buf []byte) (int, error) {
   if o.count % 10 == 9 {
      return 0, &net.OpError{Op:"read", Err:tTestClientError("log out")}
   }

   var aDlC <-chan time.Time
   if !o.readDeadline.IsZero() {
      aDl := time.NewTimer(o.readDeadline.Sub(time.Now()))
      defer aDl.Stop()
      aDlC = aDl.C
   }

   aUnit := 200 * time.Millisecond; if o.deferLogin { aUnit = 6 * time.Second }
   aTmr := time.NewTimer(aUnit)
   defer aTmr.Stop()

   var aHead map[string]interface{}
   var aData string

   select {
   case <-o.ack:
      aHead = tMsg{"Op":eAck, "Id":"n", "Type":"n"}
   case <-aTmr.C:
      o.count++
      if o.deferLogin {
         aHead = tMsg{}
      } else if o.count == 1 {
         aHead = tMsg{"Op":eLogin, "Uid":"u"+fmt.Sprint(o.id), "NodeId":fmt.Sprint(o.id)}
      } else {
         aHead = tMsg{"Op":ePost, "Id":"n", "For":[]string{"u"+fmt.Sprint(o.to)}}
         aData = fmt.Sprintf(" |msg %d|", o.count)
      }
   case <-aDlC:
      return 0, &net.OpError{Op:"read", Err:&tTimeoutError{}}
   }

   aMsg := qlib.PackMsg(aHead, []byte(aData))
   fmt.Printf("%d testclient.read %s\n", o.id, string(aMsg))
   return copy(buf, aMsg), nil
}

func (o *tTestClient) Write(buf []byte) (int, error) {
   if o.closed {
      fmt.Printf("%d testclient.write was closed\n", o.id)
      return 0, &net.OpError{Op:"write", Err:tTestClientError("closed")}
   }

   aTmr := time.NewTimer(2 * time.Second)

   select {
   case o.ack <- 1:
      aTmr.Stop()
   case <-aTmr.C:
      fmt.Printf("%d testclient.write timed out on ack\n", o.id)
      return 0, &net.OpError{Op:"write", Err:tTestClientError("noack")}
   }

   fmt.Printf("%d testclient.write got %s\n", o.id, string(buf))
   return len(buf), nil
}

func (o *tTestClient) SetReadDeadline(i time.Time) error {
   o.readDeadline = i
   return nil
}

func (o *tTestClient) Close() error {
   o.closed = true;
   time.AfterFunc(10*time.Millisecond, func(){ sTestClientId <- o.id })
   return nil
}

func (o *tTestClient) LocalAddr() net.Addr { return &net.UnixAddr{"e", "a"} }
func (o *tTestClient) RemoteAddr() net.Addr { return &net.UnixAddr{"e", "a"} }
func (o *tTestClient) SetDeadline(time.Time) error { return nil }
func (o *tTestClient) SetWriteDeadline(time.Time) error { return nil }


type tTimeoutError struct{}
func (o *tTimeoutError) Error() string   { return "i/o timeout" }
func (o *tTimeoutError) Timeout() bool   { return true }
func (o *tTimeoutError) Temporary() bool { return true }

type tTestClientError string
func (o tTestClientError) Error() string { return string(o) }

type tMsg map[string]interface{}
*/


//: these are instructions/guidance comments
//: you'll implement the public api to add/edit userdb records
//: for all ops, you look up a record in cache,
//:   and if not there call getRecord and cache the result
//:   lookups are done with aObj := o.user[Uid] (or o.alias, o.group)
//: for add/edit ops, you then modify the cache object, then call putRecord
//: locking
//:   cache read ops are done inside o.xyzDoor.RLock/RUnlock()
//:   cache add/delete ops are done inside o.xyzDoor.Lock/Unlock()
//:   tUser and tGroup object updates are done inside aObj.door.Lock/Unlock()
//: records are stored as files in subdirectories of o.root: user, alias, group
//:   user/* & group/* files are json format
//:   alias/* files are symlinks to Uid

type tUserDb struct {
   root string // top-level directory
   temp string // temp subdirectory; write files here first

   // cache records here
   userDoor sync.RWMutex
   user map[string]*tUser

   aliasDoor sync.RWMutex
   alias map[string]string // value is Uid

   groupDoor sync.RWMutex
   group map[string]*tGroup
}

type tUser struct {
   door sync.RWMutex
   Nodes map[string]tNode // value is NodeRef
   nonDefunctNodesCount int
   Aliases []tAlias // public names for the user
}

type tNode struct {
  defunct bool
  Qid string
  nodeRef int
}

type tAlias struct {
   En string // in english
   Nat string // in whatever language
}

type tGroup struct {
   door sync.RWMutex
   Uid map[string]tMember
}

type tMember struct {
   Alias string // invited/joined by this alias
   Joined bool // use a date here?
}

type tUserDbErr string
func (o tUserDbErr) Error() string { return string(o) }

type tType string
const (
   eTuser  tType = "user"
   eTalias tType = "alias"
   eTgroup tType = "group"
)

//: add a crash recovery pass on startup
//: examine temp dir
//:   complete any pending transactions
//: in transaction
//:   sync temp dir instead of data dir
//:   remove temp file in commitDir
//:   drop .tmp files

func NewUserDb(iPath string) (*tUserDb, error) {
   for _, a := range [...]tType{ "temp", eTuser, eTalias, eTgroup } {
      err := os.MkdirAll(iPath + "/" + string(a), 0700)
      if err != nil { return nil, err }
   }

   aDb := new(tUserDb)
   aDb.root = iPath+"/"
   aDb.temp = aDb.root + "temp"
   aDb.user = make(map[string]*tUser)
   aDb.alias = make(map[string]string)
   aDb.group = make(map[string]*tGroup)

   return aDb, nil
}

func (o *tUserDb) Test() error {
   //: exercise the api, print diagnostics
   //: invoke from main() before tTestClient loop; stop program if tests fail
   return nil
}

//: below is the public api
//: if same parameters are retried after success, ie data already exists,
//:   function should do nothing but return success

func (o *tUserDb) AddUser(iUid, iNewNode string) (aQid string, err error) {
   //: add user
   //: iUid not in o.user, or already has iNewNode
   //: iNewNode string is the NodeId
   //: Qid is the same as NodeId (will use hash for NodeId to generate Qid)
   //: qlib will use Qid as the name for the queue the node will attach to

   //: If iUid already exists, check if iNewNode already exists
   //: Errors-- iUid already exists, but does not have iNewNode

   // aQid = iNewNode, return aQid (don't need to use :=)

   /* ACTION PLAN
    * 1. Check if iUid is in cache. (call fetchUser, assign return value of fetchUser to a local variable)
         aUserExists := false
         if(fetchUser(iUid) != nil) {
            aUserExists = true
         }
3 possible situations: (empty user, existing user with new node, existing without new node)user is nil, user already exists, user was created (through eFetchMake --> returns existing/new user)
If user was created, you will get an empty user, not nil(only fetchCheck returns nil)
Error: existing user without new node
-check if user has any nodes:
-if yes: is one of them the new node?
-if you do not have the new node, but you have the new user, error
    * 2. If iUid already exists, check if iNewNode exists. If iNewNode does not, 
    *    exist, return error.
Check if map is empty using len(user.Nodes)
user.Nodes[iNewNode] ==> return struct ==> look at struct to see if it is valid by seeing if Qid is empty (if empty, iNewNode does not exist)
         if(aUserExists){
            for key, value := range o.user.Nodes {
               if(key==iNewNode) {
                  return error
               }
            }
         }

    * 3. Write-lock userDoor, add user to o.user 
         o.userDoor.Lock()
         o.user[iUid].Nodes = map[string]tNode{iNewNode: tNode{Defunct: false, Qid: iNewNode}}
         o.user[iUid].NonDefunctNodesCount++ // need to find someplace to initialize count to 0
    * 4. Assign iNewNode to aQid
         aQid = iNewNode
    * 5. return aQid

if aUser.Nodes[iNewNode].Qid != "" {
   return aQid
}
if len(aUser.Nodes) != 0 {
   return tUserDbErr("err msg")
}

aUser.Nodes[iNewNode] = tNode{Defunct: false, Qid: iNewNode}

    */
   
   aUser := fetchUser(iUid, eFetchMake)

   aQid = iNewNode //todo generate Qid properly

   aUser.door.Lock()
   defer aUser.door.Unlock() // do this when you return no matter what

   if len(aUser.Nodes) != 0 {
      if aUser.Nodes[iNewNode].Qid == aQid {
         return aQid
      }
   return tUserDbErr("err msg")
}

aUser.Nodes[iNewNode] = tNode{Defunct: false, Qid: aQid}
aUser.NondefunctNodesCount++

putRecord(eTuser, iUid, aUser)

return aQid
}

func (o *tUserDb) AddNode(iUid, iNode, iNewNode string) (aQid string, err error) {
   //: add node
   //: iUid has iNode
   //: iUid may already have iNewNode

   //: Can override iNewNode if it already exists.
   //: Types don't match, so change tUserDb types to accomodate the API

   //: aQid = iNewNode

   //: Error- if iUid or iNode is missing
   //: Error- if length of the map is over kUserNodeMax constant (100)

   /* ACTION PLAN
    * 1. Check if iUid exists. If iUid does not exist, return error.
    * 2. Check if iUid has iNode. If iUid does not have iNode, return error.
    * 3. Check if iUid already has iNewNode. If yes:
         aQid = iNewNode
         return aQid
    * 4. Check if the length of the map <= kUserNodeMax (100). If yes, return error.
    * 5. Lock userDoor, Add iNewNode to o.tUserDb.Nodes, set iNewNode to nondefunct, 
         increment nonDefunctNodesCount, Unlock userDoor
         o.userDoor.lock()
         o.user.Nodes[iNewNode] = tNode{defunct: false, Qid: iNewNode}
         o.user.nonDefunctNodesCount++
         o.userDoor.Unlock()
    * 6. Return aQid
         aQid = iNewNode
         return aQid   
    */
   
   /*
   aUser := fetchUser(iUid, eFetchCheck)
   aNodeQid = iNode
   aNewNodeQid = iNewNode

   if aUser == nil {
      return tUserDbErr("err msg")
   }

   aUser.door.Lock()
   defer aUser.door.Unlock()

   if aUser.Nodes[iNode].Qid != aNodeQid {
      return tUserDbErr("err msg")
   }
   if aUser.Nodes[iNewNode].Qid == aNewNodeQid {
      return aNewNodeQid
   }
   if aUser.NondefunctNodesCount == kUserNodeMax {
      return tUserDbErr("err msg")
   }

   aUser.Nodes[iNewNode] = tNode{Defunct: false, Qid: aNewNodeQid}
   aUser.NondefunctNodes++

   return aNewNodeQid

    */

   return "", nil
}

func (o *tUserDb) DropNode(iUid, iNode string) (aQid string, err error) {
   //: mark iNode defunct
   //: iUid has iNode

   //: Check if iUid has iNode.
   //: Check if iNode is already defunct. If it is, return success.
   //: Check if there are 2 non-defunct nodes in iUid. If no, error. If yes, defunct that node.

   //: Requires change of tUserDb. Nodes will be map[string]struct, struct contains string Qid and bool defunct
   //: Define new type- tNode

   //: When assigning to Nodes map, make a compound literal tNode that changes either the string Qid or the bool defunct
   //: (will have copy of one value and one changed value)

   //: Error- iUid does not have iNode
   //: Error- you only have one node left
   
   /* ACTION PLAN
    * 1. Check if iUid has iNode. If not, return error.
    * 2. Check if iNode is already defunct. If it is:
         aQid = iNode
         return aQid
    * 3. Lock userDoor, make iNode in Nodes map defunct, decrement nonDefunctNodesCount
         o.userDoor.Lock()
         o.user.Nodes[iNode] = tNode{defunct: true, Qid: iNode}
         o.user.nonDefunctNodesCount--
         o.userDoor.Unlock()
    * 4. aQid = iNode
         return aQid
    */
   
   /* CODE
   aUser := fetchUser(iUid, eFetchCheck)
   aUser.door.Lock()
   defer aUser.door.Unlock()

   aQid = iNode
   if aUser.Nodes[iNode].Qid != aQid {
      return tUserDbErr("err msg")
   }
   if aUser.Nodes[iNode].Defunct {
      return aQid
   }

   aUser.Nodes[iNode] = tNode{Defunct: true, Qid: iNode}
   aUser.NondefunctNodesCount--
   return aQid

    */

   return "", nil
}

func (o *tUserDb) AddAlias(iUid, iNode, iNat, iEn string) error {
   //: add aliases to iUid and o.alias
   //: iUid has iNode
   //: iNat != iEn, iNat or iEn != "" <-- could optimize this code later (write it in 1 line)

   //: Error- Alias exists in o.alias
   //: Error- Aliax belongs to someone else
   
   /* ACTION PLAN
    * 1. Check if iUid has iNode. If not, return error.
    * 2. Read lock o.alias, Check if iNat and iEn exist in o.alias. If yes, return error.
    * 3. Write-lock o.alias, add iNat and iEn into o.alias
    * 4. Write-lock user, add iNat and iEn into user's map of aliases 
    */

   return nil
}

func (o *tUserDb) DropAlias(iUid, iNode, iAlias string) error {
   //: mark alias defunct in o.alias
   //: iUid has iNode
   //: iAlias for iUid

   //: In alias index, create a struct with uId & defunct flag
   //: Only the user who owns the alias can drop the alias, so you must verify that the user & node are correct
   //: Need defunct flag in tAlias
   //: Have another struct for index value for o.tAlias index, which also has defunct
   //: Call tAlias --> tUserAlias, new struct: tAlias contains string uId & bool defunct

   //: Error-- iNode or iAlias don't belong to user
   
  /* ACTION PLAN
   * 1. Check if iNode belongs to iUid. If not, return error.
   * 2. Check if iAlias belongs to iUid. If not, return error.
   * 3. Find iAlias in user's map of aliases, and mark it as defunct.
   */

   return nil
}

//func (o *tUserDb) DropUser(iUid string) error { return nil }

func (o *tUserDb) Verify(iUid, iNode string) (aQid string, err error) {
   //: return Qid of node
   //: iUid has iNode
   // trivial implementation for qlib testing

   // Check if the node is defunct

   if o.user[iUid] != nil && o.user[iUid].Nodes[iNode] != 0 {
      return "q"+iNode, nil
   }
   return "", tUserDbErr("no such user/node")
}

func (o *tUserDb) GetNodes(iUid string) (aQids []string, err error) {
   //: return Qids for iUid
   // trivial implementation for qlib testing

   // Error-- uId does not exist
   // Cannot return defunct nodes!

   for aN,_ := range o.user[iUid].Nodes { // must do appropriate locking
      // check if the node is defunct
      aQids = append(aQids, aN)
   }
   return aQids, nil
}

func (o *tUserDb) Lookup(iAlias string) (aUid string, err error) {
   //: return uid for iAlias
   //: Error-- iAlias does not exist, or iAlias is defunct
   
   /* ACTION PLAN
    * 1. Check if iAlias exists in map of aliases. If iAlias exists, check if is defunct.
         Return error if it is defunct. Return error if iAlias does not exist.
    * 2. RLock aliasDoor, assign aUid to the Uid iAlias refers to.
         o.aliasDoor.RLock()
         aUid = alias[iAlias]
    * 3. return aUid
    */
   return "", nil
}

func (o *tUserDb) GroupInvite(iGid, iAlias, iByAlias, iByUid string) error {
   //: add member to group, possibly create group
   //: iAlias exists
   //: iGid exists, iByUid in group, iByAlias ignored
   //: iGid !exists, make iGid and add iByUid with iByAlias
   //: iByAlias for iByUid

   // may want to create helper function that verifies an alias (or call lookup)
   // could call lookup to check if iByAlias matches with iByUid, and to check if iByAlias exists

   //: iByAlias is optional
   //: Error-- group is created, but iByAlias is not given
   
  /* ACTION PLAN
   * 1. Check if iAlias exists. If not, send error.
   * 2. Check if iByAlias exists. If not, send error.
   * 3. Call fetchGroup with eFetchMake to see if group with iGid exists
        (group with iGid will be created if it doesn't)
   * 4. If iByUid is not in group, add iByUid into the group (with status joined)
        along with iByAlias if provided
   * 5. Add the user with iAlias into the group, make status invited.  
    */
   return nil
}

/*
type tStatus int 
const (_ tStatus = iota; eStatInvited; eStatBarred) 
*/
func (o *tUserDb) GroupJoin(iGid, iUid, iNewAlias string) error {
   //: set joined status for member
   //: iUid in group
   //: iNewAlias optional for iUid

   //: iNewAlias must match with iUid
   // if not alias, iUid uses the alias they were invited by

   // Error-- iGid does not exist, iUid is not in group, iNewAlias does not match iUid

   /* ACTION PLAN
    * 1. Check if iGid exists. If not, return error.
    * 2. Check if iUid is in group. If not, return error.
    * 3. Check if iNewAlias is an alias for iUid. If not, return error.
    * 4. Change status for iUid to joined.
    */
   
    /*
    aGroup := fetchGroup(iGid, eFetchCheck)
    aUser := fetchUser(iUid, eFetchCheck)
     // CHECK ERROR CASES
    if aGroup == nil { // if group does not exist
      return tUserDbErr("err msg")
    }
    if aGroup.Uid[iGid].joined != eStatJoined { // check if iUid is already in group
      return tUserDbErr("err msg")
    }
    // check if iUid has iNewAlias (this could be wrong)

    aUser.door.RLock()
    defer aUser.door.RUnlock()
    aHasAlias := false
    for int i=0; i < len(aUser.Aliases); i++ {
      if aUser.Alias[i] == iNewAlias {
        aHasAlias = true
        break
      }
    }
    if !aHasAlias {
      return tUserDbErr("err msg")
    }

    //CHANGE STATUS TO JOINED
    aGroup.Uid[iUid].Status = eStatJoined
    */
    
   return nil
}

func (o *tUserDb) GroupAlias(iGid, iUid, iNewAlias string) error {
   //: update member alias
   //: iUid in group
   //: iNewAlias for iUid
   
   /* ACTION PLAN
    * 1. Check if iUid is in iGid. If not, return error.
    * 2. Check if iNewAlias belongs to iUid. If not, return error.
    * 3. For the group, change iUid's alias to iNewAlias.
    */
   return nil
}

func (o *tUserDb) GroupDrop(iGid, iUid, iByUid string) error {
   //: change member status of member with iUid
   //: iUid in group, iByUid same or in group
   //: iUid == iByUid, status=invited
   //: iUid != iByUid, if iUid status==joined, status=barred
   // do not drop member
   
   /* ACTION PLAN
    * 1. Check if iUid is in iGid. If not, return error.
    * 2. Check if iUid == iByUid. If yes, set iUid's status to invitied.
    * 3. If iUid != iByUid, change iUid's status to barred.
    */
   return nil
}

func (o *tUserDb) GroupGetUsers(iGid, iByUid string) (aUids []string, err error) {
   //: return uids in iGid
   //: iByUid is member
   for a,_ := range o.group["g1"].Uid {
      // check if each user's status is joined
      aUids = append(aUids, a)
   }
   return aUids, nil
}


func (o *tUserDb) fetchUser(iUid string) *tUser {
   o.userDoor.RLock() // read-lock user map
   aUser := o.user[iUid] // lookup user in map
   o.userDoor.RUnlock()

   if aUser == nil { // user not in cache
      aObj, err := o.getRecord(eTuser, iUid) // lookup user on disk
      if err != nil { panic(err) }
      aUser = aObj.(*tUser) // "type assertion" to extract *tUser value from interface{}
      aUser.door.Lock() // write-lock user

      o.userDoor.Lock() // write-lock user map
      o.user[iUid] = aUser // add user to map
      o.userDoor.Unlock()
   } else {
      aUser.door.Lock() // write-lock user
   }
   return aUser // user is write-locked and cached
   // must do putRecord() and .door.Unlock() on return value after changes!
}

/*
func (o *tUserDb) fetchGroup(iGid string, iMake tFetch) (*tGroup, error){
  o.groupDoor.RLock() // read-lock group map
  aGroup := o.group[iGid] // lookup group in map
  o.groupDoor.RUnlock()

  if aGroup == nil { // group not in cache
    aObj, err := o.getRecord(eTgroup, iGid) 
    if err!= nil {return nil, err}
    aGroup = aObj.(*tGroup) // "type assertion" to extract *tGroup value

    if aGroup.Uid == nil { // group does not exist
      if !iMake {
        return nil, nil
      }
      aGroup.Uid = make(map[string]tMember) // initialize map of members
    }

    o.groupDoor.Lock()
    if aTemp:=o.group[iGid]; aTemp != nil { // recheck the map
       aGroup = aTemp
    } else {
       o.group[iGid] = aGroup // add group to map
    }
    o.groupDoor.Unlock()
   }
   return aGroup, nil
}
*/

// pull a file into a cache object
func (o *tUserDb) getRecord(iType tType, iId string) (interface{}, error) {
   var err error
   var aObj interface{}
   aPath := o.root + string(iType) + "/" + iId

   // in case putRecord was interrupted
   err = os.Link(aPath + ".tmp", aPath)
   if err != nil {
      if !os.IsExist(err) && !os.IsNotExist(err) { return nil, err }
   } else {
      fmt.Println("getRecord: finished transaction for "+aPath)
   }

   switch (iType) {
   default:
      panic("getRecord: unexpected type "+iType)
   case eTalias:
      aLn, err := os.Readlink(aPath)
      if err != nil {
         if os.IsNotExist(err) { return nil, nil }
         return nil, err
      }
      return &aLn, nil
   case eTuser:  aObj = &tUser{}
   case eTgroup: aObj = &tGroup{}
   }

   aBuf, err := ioutil.ReadFile(aPath)
   if err != nil {
      if os.IsNotExist(err) { return aObj, nil }
      return nil, err
   }

   err = json.Unmarshal(aBuf, aObj)
   return aObj, err
}

// save cache object to disk. getRecord must be called before this
func (o *tUserDb) putRecord(iType tType, iId string, iObj interface{}) error {
   var err error
   aPath := o.root + string(iType) + "/" + iId
   aTemp := o.temp + string(iType) + "_" + iId

   err = os.Remove(aPath + ".tmp")
   if err == nil {
      fmt.Println("putRecord: removed residual .tmp file for "+aPath)
   }

   switch (iType) {
   default:
      panic("putRecord: unexpected type "+iType)
   case eTalias:
      err = os.Symlink(iObj.(string), aPath + ".tmp")
      if err != nil { return err }
      return o.commitDir(iType, aPath)
   case eTuser, eTgroup:
   }

   aBuf, err := json.Marshal(iObj)
   if err != nil { return err }

   aFd, err := os.OpenFile(aTemp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
   if err != nil { return err }
   defer aFd.Close()

   for aPos,aLen := 0,0; aPos < len(aBuf); aPos += aLen {
      aLen, err = aFd.Write(aBuf[aPos:])
      if err != nil { return err }
   }

   err = aFd.Sync()
   if err != nil { return err }

   err = os.Link(aTemp, aPath + ".tmp")
   if err != nil { return err }
   err = os.Remove(aTemp)
   if err != nil { return err }

   return o.commitDir(iType, aPath)
}

// sync the directory and set the filename
func (o *tUserDb) commitDir(iType tType, iPath string) error {
   aFd, err := os.Open(o.root + string(iType))
   if err != nil { return err }
   defer aFd.Close()
   err = aFd.Sync()
   if err != nil { return err }

   err = os.Remove(iPath)
   if err != nil && !os.IsNotExist(err) { return err }
   err = os.Rename(iPath + ".tmp", iPath)
   return err
}
