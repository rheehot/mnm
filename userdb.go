package main

import (
   "fmt"
   "io/ioutil"
   "encoding/json"
   "os"
   "sync"
   "strconv"
)


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
   Nodes map[string]tNode
   NonDefunctNodesCount int
   Aliases []tAlias // public names for the user
}

type tNode struct {
  Defunct bool
  Qid string
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

type tUserDbErr struct {
   msg string
   id int
}

func (o tUserDbErr) Error() string { return string(o.msg) }
const ( _= iota; eErrUserInvalid; eErrMissingNode; eErrNodeInvalid; eErrMaxNodes; eErrLastNode )

type tType string
const (
   eTuser  tType = "user"
   eTalias tType = "alias"
   eTgroup tType = "group"
)

const kUserNodeMax = 100

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

func TestUserDb() {
   //: exercise the api, print diagnostics
   //: invoke from main() before tTestClient loop; stop program if tests fail
   _ = os.RemoveAll("store-udb-test")
   aDb, err := NewUserDb("store-udb-test")

   //defer os.RemoveAll(aDb.root)

   aOk := true

   fReport := func(cMsg string) {
      aOk = false
      if err != nil {
         fmt.Printf("%s: %s\n", cMsg, err.Error())
      } else {
         fmt.Printf(cMsg + "\n")
      }
   }

   aUid, aUid2, aUid3 := "User1", "User2", "User3"
   aNode1, aNode2 := "Node1", "Node2"

   // IMPLEMENTING ADDUSER
   fmt.Println("Testing AddUser") // todo drop this
   // testing successful case
   _, err = aDb.AddUser(aUid, aNode1)
   if err != nil || aDb.user[aUid].Nodes[aNode1].Qid != aNode1 {
      fReport("AddUser normal case failed")
   }
   _, err = aDb.AddUser(aUid, aNode1)
   if err != nil || aDb.user[aUid].Nodes[aNode1].Qid != aNode1 {
      fReport("AddUser normal case failed")
   }
   // testing error cases
   _, err = aDb.AddUser(aUid, aNode2) // iUid that already exists
   if err == nil {
      fReport("AddUser error expected, got success")
   }
   fmt.Println("AddUser tests complete") // todo drop this
   fmt.Println()

   // IMPLEMENTING ADDNODE
   fmt.Println("Testing AddNode") // todo drop this
   //testing successful case
   _, err = aDb.AddNode(aUid, aNode1, aNode2)
   if err != nil || aDb.user[aUid].Nodes[aNode2].Qid != aNode2 {
      fReport("AddNode normal case failed")
   }
   _, err = aDb.AddNode(aUid, aNode1, aNode2) //repeate add  Node2
   if err != nil || aDb.user[aUid].Nodes[aNode2].Qid != aNode2 {
      fReport("AddNode normal case failed")
   }
   // test error cases for AddNode
   _, err = aDb.AddNode(aUid, "nodeOne", aNode1) //iNode invalid
   if err == nil {
      fReport("AddNode error expected for invalid iNode, got success")
   }
   _, err = aDb.AddNode(aUid, aNode1, aNode1) // iNode == iNewNode
   if err == nil {
      fReport("AddNode error expected for iNode==iNewNode, got success")
   }

   // Try to add more than 100 nodes into a user
   aNewiNode := "Node0"
   aDb.AddUser(aUid2, aNewiNode)
   for i := 1; i < 100; i++ {
      aNewiNode = "Node" + strconv.Itoa(i)
      aDb.AddNode("aUid2", "Node0", aNewiNode)
   }
   _, err = aDb.AddNode("aUid2", "Node0", "Node101")
   if err == nil {
      fReport("AddNode error expected for adding >100 nodes, got success")
   }
   fmt.Println("AddNode tests complete") // todo drop this
   fmt.Println()

   // IMPLEMENTING DROPNODE
   fmt.Println("Testing DropNode") //todo drop this
   // testing successful case
   _, err = aDb.DropNode(aUid, aNode1) // successful case
   if err != nil || ! aDb.user[aUid].Nodes[aNode1].Defunct {
      fReport("DropNode: Error with successful case")
   }

   _, err = aDb.DropNode(aUid, aNode1) // successful case (repeated)
   if err != nil || ! aDb.user[aUid].Nodes[aNode1].Defunct {
      fReport("DropNode: Node should already have been dropped, error with successful case")
   }

   // testing error cases
   _, err = aDb.DropNode("Non-existant user", "Node1") // error: user does not exist
   if err == nil {
      fReport("DropNode: user does not exist, error expected")
   }
   _, err = aDb.DropNode(aUid, "Non-existant node") // error: node does not exist
   if err == nil {
      fReport("DropNode: Node does not exist, error expected")
   }
   // error: only one node left
   aDb.AddUser("User3", "Node1")
   _, err = aDb.DropNode(aUid3, aNode1)
   if err == nil {
      fReport("DropNode: last node dropped, error expected")
   }
   fmt.Println("DropNode tests complete") //todo drop this

   if aOk {
      fmt.Println("UserDb tests passed")
   }
}

//: below is the public api
//: if same parameters are retried after success, ie data already exists,
//:   function should do nothing but return success

func (o *tUserDb) AddUser(iUid, iNewNode string) (aQid string, err error) {
   //: add user
   //: iUid not in o.user, or already has iNewNode
   aUser, err := o.fetchUser(iUid, eFetchMake)
   if err != nil { panic(err) }

   aQid = iNewNode //todo generate Qid properly

   aUser.door.Lock()
   defer aUser.door.Unlock()

   if len(aUser.Nodes) != 0 {
      if aUser.Nodes[iNewNode].Qid != aQid {
         return "", tUserDbErr{msg: fmt.Sprintf("AddUser: Uid %s found, Node %s missing", iUid, iNewNode), id: eErrMissingNode}
      }
      return aQid, nil
   }

   aUser.Nodes[iNewNode] = tNode{Defunct: false, Qid: aQid}
   aUser.NonDefunctNodesCount++

   err = o.putRecord(eTuser, iUid, aUser)
   if err != nil { panic(err) }
   return aQid, nil
}

func (o *tUserDb) AddNode(iUid, iNode, iNewNode string) (aQid string, err error) {
   //: add node
   //: iUid has iNode
   //: iUid may already have iNewNode
   aUser, err := o.fetchUser(iUid, eFetchCheck)
   if err != nil { panic(err) }

   if aUser == nil { // if user does not exist
      return "", tUserDbErr{msg: fmt.Sprintf("AddNode: Uid %s not found", iUid), id: eErrUserInvalid}
   }

   aUser.door.Lock()
   defer aUser.door.Unlock()

   aNodeQid := iNode
   aNewNodeQid := iNewNode
   if iNode == iNewNode {
      return "", tUserDbErr{msg: fmt.Sprintf("AddNode: iNode cannot equal iNewNode"), id: eErrNodeInvalid}
   }
   if aUser.Nodes[iNode].Qid != aNodeQid { // unexpected value of Qid for iNode (iNode invalid)
      return "", tUserDbErr{msg: fmt.Sprintf("AddNode: iNode %s invalid", iNode), id: eErrNodeInvalid}
   }
   if aUser.Nodes[iNewNode].Qid == aNewNodeQid { // expected value of Qid for iNode (iNewNode already exists)
      return aNewNodeQid, nil
   }
   if aUser.NonDefunctNodesCount == kUserNodeMax { // cannot add more nodes
      return "", tUserDbErr{msg: "AddNode: Exceeds max nodes", id: eErrMaxNodes}
   }

   aUser.Nodes[iNewNode] = tNode{Defunct: false, Qid: aNewNodeQid}
   aUser.NonDefunctNodesCount++
   err = o.putRecord(eTuser, iUid, aUser)
   if err != nil { panic(err) }
   return aNewNodeQid, nil
}

func (o *tUserDb) DropNode(iUid, iNode string) (aQid string, err error) {
   //: mark iNode defunct
   //: iUid has iNode
   aUser, err := o.fetchUser(iUid, eFetchCheck) // need to check if user is valid, if not, return error
   if err != nil { panic(err) }

   if aUser == nil { // error: user does not exist
      return "", tUserDbErr{msg: fmt.Sprintf("DropNode: user not found"), id: eErrUserInvalid}
   }
   aUser.door.Lock()
   defer aUser.door.Unlock()

   aQid = iNode
   if aUser.Nodes[iNode].Qid != aQid { // error: node is invalid
      return "", tUserDbErr{msg: fmt.Sprintf("DropNode: iNode %s invalid", iNode), id: eErrNodeInvalid}
   }
   if aUser.Nodes[iNode].Defunct { // node has already been marked defunct
      return aQid, nil
   }
   if aUser.NonDefunctNodesCount <= 1 { // error: user only has one node left
      return "", tUserDbErr{msg: fmt.Sprintf("DropNode: cannot drop last node"), id: eErrLastNode}
   }

   aUser.Nodes[iNode] = tNode{Defunct: true, Qid: iNode}
   aUser.NonDefunctNodesCount--

   o.putRecord(eTuser, iUid, aUser)
   if err != nil { panic(err) }
   return aQid, nil
}

func (o *tUserDb) AddAlias(iUid, iNode, iNat, iEn string) error {
   //: add aliases to iUid and o.alias
   //: iUid has iNode
   //: iNat != iEn, iNat or iEn != ""
   return nil
}

func (o *tUserDb) DropAlias(iUid, iNode, iAlias string) error {
   //: mark alias defunct in o.alias
   //: iUid has iNode
   //: iAlias for iUid
   return nil
}

//func (o *tUserDb) DropUser(iUid string) error { return nil }

func (o *tUserDb) Verify(iUid, iNode string) (aQid string, err error) {
   //: return Qid of node
   //: iUid has iNode
   return aQid, nil
}

func (o *tUserDb) GetNodes(iUid string) (aQids []string, err error) {
   //: return Qids for iUid
   return aQids, nil
}

func (o *tUserDb) Lookup(iAlias string) (aUid string, err error) {
   //: return uid for iAlias
   return aUid, nil
}

func (o *tUserDb) GroupInvite(iGid, iAlias, iByAlias, iByUid string) (aUid string, err error) {
   //: add member to group, possibly create group
   //: iAlias exists
   //: iGid exists, iByUid in group, iByAlias ignored
   //: iGid !exists, make iGid and add iByUid with iByAlias
   //: iByAlias for iByUid
   return aUid, nil
}

func (o *tUserDb) GroupJoin(iGid, iUid, iNewAlias string) (aAlias string, err error) {
   //: set joined status for member
   //: iUid in group
   //: iNewAlias optional for iUid
   return aAlias, nil
}

func (o *tUserDb) GroupAlias(iGid, iUid, iNewAlias string) (aAlias string, err error) {
   //: update member alias
   //: iUid in group
   //: iNewAlias for iUid
   return aAlias, nil
}

func (o *tUserDb) GroupDrop(iGid, iAlias, iByUid string) (aUid string, err error) {
   //: iAlias in group, iByUid same or in group
   //: iAlias -> iByUid, status=invited
   //: otherwise, if iAlias status==joined, status=barred else delete member
   return aUid, nil
}

func (o *tUserDb) GroupGetUsers(iGid, iByUid string) (aUids []string, err error) {
   //: return uids in iGid
   //: iByUid is member
   return aUids, nil
}

type tFetch bool
const eFetchCheck, eFetchMake tFetch = false, true

// retrieve user from cache or disk
func (o *tUserDb) fetchUser(iUid string, iMake tFetch) (*tUser, error) {
   o.userDoor.RLock() // read-lock user map
   aUser := o.user[iUid] // lookup user in map
   o.userDoor.RUnlock()

   if aUser == nil { // user not in cache
      aObj, err := o.getRecord(eTuser, iUid) // lookup user on disk
      if err != nil { return nil, err }
      aUser = aObj.(*tUser) // "type assertion" extracts *tUser from interface{}

      if aUser.Nodes == nil { // user not on disk
         if !iMake {
            return nil, nil
         }
         aUser.Nodes = make(map[string]tNode) // initialize user
      }

      o.userDoor.Lock() // write-lock user map
      if aTemp := o.user[iUid]; aTemp != nil { // recheck the map
         aUser = aTemp
      } else {
         o.user[iUid] = aUser // add user to map
      }
      o.userDoor.Unlock()
   }
   return aUser, nil // do .door.[R]Lock() on return value before use
}

func (o *tUserDb) fetchGroup(iGid string, iMake tFetch) (*tGroup, error){
   o.groupDoor.RLock() // read-lock group map
   aGroup := o.group[iGid] // lookup group in map
   o.groupDoor.RUnlock()

   if aGroup == nil { // group not in cache
      aObj, err := o.getRecord(eTgroup, iGid)
      if err != nil { return nil, err }
      aGroup = aObj.(*tGroup) // "type assertion" to extract *tGroup value

      if aGroup.Uid == nil { // group does not exist
         if !iMake {
            return nil, nil
         }
         aGroup.Uid = make(map[string]tMember) // initialize map of members
      }

      o.groupDoor.Lock()
      if aTemp := o.group[iGid]; aTemp != nil { // recheck the map
         aGroup = aTemp
      } else {
         o.group[iGid] = aGroup // add group to map
      }
      o.groupDoor.Unlock()
   }
   return aGroup, nil
}

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

   switch iType {
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

   switch iType {
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
