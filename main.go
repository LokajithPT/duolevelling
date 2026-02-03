package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
)

type Payload struct {
	Filename string `json:"filename"`
	Typed    string `json:"typed"`
	Pasted   string `json:"pasted"`
	Real     string `json:"real"`
}

type CheckMeRequest struct {
	ProjectID string `json:"project_id"`
	TaskID    string `json:"task_id"`
}

type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Done        bool   `json:"status"`
}

type Project struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Tasks []Task `json:"tasks"`
}

type ProjectStore struct {
	Projects []Project `json:"projects"`
}

type Streak struct {
	CurrentStreak int    `json:"current_streak"`
	LastCompleted string `json:"last_completed"`
	StreakFreeze  int    `json:"streak_freeze"`
}

var streak Streak

var projectStore ProjectStore

var submissions = make(map[string]Payload)

func main() {
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/ping", pingHandler)
	http.HandleFunc("/submit", submit)
	http.HandleFunc("/dashboard", dashboard)

	http.HandleFunc("/task-done", taskDoneHandler)
	http.HandleFunc("/checkme", checkme)

	err := loadSubmissions()
	if err != nil {
		log.Fatal("failed to load data:", err)
	}

	loadProjects()
	loadStreak()

	http.HandleFunc("/api/projectlist", projectListHandler)
	http.HandleFunc("/api/tasklist", taskListHandler)
	http.HandleFunc("/api/abouttask", aboutTaskHandler)
	http.HandleFunc("/taskstatus", taskStatusHandler)
	http.HandleFunc("/streakstatus", streakStatusHandler)

	fmt.Println("runnin the server in 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))

}

// helpers and shit
// projects
func loadProjects() error {
	file, err := os.Open("projects.json")
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(&projectStore)
}

func saveProjects() error {
	file, err := os.Create("projects.json")
	if err != nil {
		return err
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	return enc.Encode(projectStore)
}

// streak
func loadStreak() error {
	file, err := os.Open("streak.json")
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(&streak)
}

func saveStreak() error {
	file, err := os.Create("streak.json")
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(streak)
}

// json thing to store in the server
const dataFile = "data.json"

func saveSubmissions() error {
	file, err := os.Create(dataFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // pretty JSON 😌
	return encoder.Encode(submissions)
}

func loadSubmissions() error {
	file, err := os.Open(dataFile)
	if err != nil {
		// file doesn't exist yet — totally fine
		return nil
	}
	defer file.Close()

	return json.NewDecoder(file).Decode(&submissions)
}

//routes

func submit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed ", http.StatusMethodNotAllowed)
		return

	}

	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "expected application/json ", http.StatusBadRequest)
		return

	}

	var payload Payload

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {

		http.Error(w, "invalid json:"+err.Error(), http.StatusBadRequest)
		return
	}

	submissions[payload.Filename] = payload

	err := saveSubmissions()
	if err != nil {
		http.Error(w, "failed to save data ", http.StatusInternalServerError)
		return
	}

	//logging for now
	fmt.Println("---submission---")
	fmt.Println("file:", payload.Filename)
	fmt.Println("typed chars:", (payload.Typed))
	fmt.Println("pasted chars:", (payload.Pasted))
	fmt.Println("real chars:", (payload.Real))

	//respond
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "received",
	})

}

func taskDoneHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		TaskID string `json:"task_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if body.TaskID == "" {
		http.Error(w, "task_id required", http.StatusBadRequest)
		return
	}

	// just pick it up
	fmt.Println("TASK DONE:", body.TaskID)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("welcome to duo ")
}

func pingHandler(w http.ResponseWriter, r *http.Request) {

	fmt.Fprintln(w, "pong")
	fmt.Println(w, "pong")
}

func dashboard(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/dashboard.html")
	if err != nil {
		http.Error(w, "template error ", http.StatusInternalServerError)
		fmt.Println("template error ")
		return

	}

	err = tmpl.Execute(w, submissions)
	if err != nil {
		http.Error(w, "render error ", http.StatusInternalServerError)
		fmt.Println("render error ", err)

	}
}

//api which will send data

func projectListHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(projectStore)
}

//tasklist

func taskListHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID string `json:"project_id"`
		TaskID    string `json:"task_id"`
		Done      bool   `json:"done"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}

	for pi, p := range projectStore.Projects {
		if p.ID == req.ProjectID {
			for ti, t := range p.Tasks {
				if t.ID == req.TaskID {
					projectStore.Projects[pi].Tasks[ti].Done = req.Done
					saveProjects()
					w.Write([]byte("ok"))
					return
				}
			}
		}
	}

	http.Error(w, "task not found", 404)
}

// abouttask
func aboutTaskHandler(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project")
	tid := r.URL.Query().Get("task")

	for _, p := range projectStore.Projects {
		if p.ID == pid {
			for _, t := range p.Tasks {
				if t.ID == tid {
					json.NewEncoder(w).Encode(t)
					return
				}
			}
		}
	}

	http.Error(w, "task not found", 404)
}

// taskstatus
func taskStatusHandler(w http.ResponseWriter, r *http.Request) {
	status := make(map[string]map[string]bool)

	for _, p := range projectStore.Projects {
		status[p.ID] = make(map[string]bool)
		for _, t := range p.Tasks {
			status[p.ID][t.ID] = t.Done
		}
	}

	json.NewEncoder(w).Encode(status)
}

// streakstatus
func streakStatusHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(streak)
}

//checking from todo to checking

func checkme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CheckMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	data, err := loadProjects()
	if err != nil {
		http.Error(w, "failed to load projects", 500)
		return
	}

	for pi, project := range data.Projects {
		if project.ID == req.ProjectID {
			for ti, task := range project.Tasks {
				if task.ID == req.TaskID {

					if task.Status == "todo" {
						data.Projects[pi].Tasks[ti].Status = "checking"
						saveProjects(data)
						fmt.Fprintln(w, "submitted for checking")
						return
					}

					fmt.Fprintln(w, "already submitted")
					return
				}
			}
		}
	}

	http.Error(w, "task not found", 404)
}
