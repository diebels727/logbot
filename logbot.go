package logbot

import(
  // "fmt"
  "github.com/diebels727/go-ircevent"
  "time"
  "fmt"
  "os"
  "path"
  "path/filepath"
  "strings"
  // "io/ioutil"
  // "strconv"
  // "syscall"
  "github.com/nightlyone/lockfile"
  "code.google.com/p/go-sqlite/go1/sqlite3"
)

type LogBot struct {
  Server string
  Channel string
  Port string
  Botname string
  Username string
}

func path_exists(path string) (bool, error) {
    _, err := os.Stat(path)
    if err == nil { return true, nil }
    if os.IsNotExist(err) { return false, nil }
    return false, err
}

func check(e error) {
    if e != nil {
        panic(e)
    }
}

func New(server string,channel string,port string,botname string,username string) (*LogBot) {
  return &LogBot{server,channel,port,botname,username}
}


func (l *LogBot) RunAndLoop() {
  sql := make(chan string)  //this will be buffered

  //replace all instances of .'s
  server_slug := strings.Replace(l.Server,".","-",-1)
  //remove #'s and .'s from channel
  channel_slug := strings.Replace(l.Channel,"#","",-1)
  channel_slug = strings.Replace(channel_slug,".","-",-1)

  //lockfile so we do no harm
  log_path := path.Join(server_slug,channel_slug)
  abs_log_path,err := filepath.Abs(log_path)
  if err != nil {
    fmt.Println("Log path absolute error.")
    return
  }
  file_mode := os.FileMode(0777)
  err = os.MkdirAll(log_path,file_mode)
  if err != nil {
    fmt.Println("Cannot create log path.")
    return
  }
  lock_file,err := lockfile.New(path.Join(abs_log_path,"lock"))
  if err != nil {
    fmt.Println("Cannot get lock, aborting...")
    return
  }
  err = lock_file.TryLock()
  if err != nil {
    fmt.Println("Process locked, aborting...")
    return
  }

  //persistence
  db_file := "db.sqlite3"
  db_path := path.Join(server_slug,channel_slug,db_file)
  db, err := sqlite3.Open(db_path)
  if err != nil {
    fmt.Println("Cannot get handle to DB, aborting...")
    return
  }
  defer db.Close()
  db.Exec(`CREATE TABLE events(
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER,
    username VARCHAR(64),
    host VARCHAR(255),
    message TEXT
  );`)

  //connect to IRC
  con := irc.IRC(l.Username,l.Botname)
  con.Debug = true
  con.VerboseCallbackHandler = true
  err = con.Connect(l.Server + ":" + l.Port)
  if err != nil {
    fmt.Println("Cannot connect to server, aborting...")
    return
  }

  con.AddCallback("001", func (e *irc.Event) {
    channel := l.Channel
    byte_channel := []byte(channel)
    //prepend hash if not hashed
    if !(byte_channel[0] == []byte("#")[0]) {
      channel = "#"+channel
    }
    con.Join(channel)
  })

  con.AddCallback("PRIVMSG", func (e *irc.Event) {
    t := time.Now().Unix()
    t_str := fmt.Sprintf("%d",t)
    sql_statement := "INSERT INTO events(timestamp,username,host,message) VALUES("+"\""+t_str+"\""+","+"\""+e.Nick+"\""+","+"\""+e.Host+"\""+","+"\""+e.Message()+"\""+");"
    sql <- sql_statement
  })

  //this is an event for receiving a channel from a channel list
  //each channel listed triggers an event
  con.AddCallback("322",func(e *irc.Event) {

  })

  //this event is triggered when the channel list is done (and perhaps when it begins)
  con.AddCallback("323",func(e *irc.Event) {
    con.Join(l.Channel)
  })

  go func(sql chan string) {
    for {
      statement := <- sql
      db.Exec(statement)
    }
  }(sql)

  con.Loop()
}
