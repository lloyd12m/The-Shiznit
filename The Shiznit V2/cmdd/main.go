package main

import (
	"automg-go/internal/insta"
	"automg-go/internal/ui"
	"automg-go/internal/utils"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	mathrand "math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	// Change these three values for each build/user subscription.
	subscriptionYear  = 2026
	subscriptionMonth = 9
	subscriptionDay   = 30

	trustedTimeAPI = "https://timeapi.io/api/Time/current/zone?timeZone=UTC"
)

var (
	users    []string
	sessions []string
	proxies  []string

	claimedTargets sync.Map // تتبع اليوزرات التي صيدت بنجاح لمنع تكرارها بين الثريدات

	sessionsEmptyAlerted sync.Once

	subscriptionExpiry = time.Date(subscriptionYear, subscriptionMonth, subscriptionDay, 0, 0, 0, 0, time.Local)

	randomMu sync.Mutex
	rng      = mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
)

type lineCycler struct {
	name  string
	mu    sync.Mutex
	items []string
}

func (q *lineCycler) next() (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		lines, err := utils.ReadFileLines(q.name)
		if err != nil || len(lines) == 0 {
			return "", fmt.Errorf("%s is empty", q.name)
		}
		q.items = append(q.items, lines...)
	}
	v := q.items[0]
	q.items = q.items[1:]
	return v, nil
}

func nextClaimTarget(queue <-chan string) (string, error) {
	select {
	case target := <-queue:
		if strings.TrimSpace(target) != "" {
			return target, nil
		}
		return "", fmt.Errorf("empty target")
	default:
		return "", fmt.Errorf("no target available")
	}
}

func getTrustedTime() (time.Time, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(trustedTimeAPI)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("time API returned status %s", resp.Status)
	}

	var data struct {
		DateTime string `json:"dateTime"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return time.Time{}, err
	}

	trustedTime, err := time.Parse(time.RFC3339Nano, data.DateTime+"Z")
	if err != nil {
		return time.Time{}, err
	}
	return trustedTime.UTC(), nil
}

func checkSubscription() bool {
	trustedNow, err := getTrustedTime()
	if err != nil {
		fmt.Printf("Unable to verify subscription time online: %v\n", err)
		return false
	}

	if !trustedNow.Before(subscriptionExpiry.UTC()) {
		ui.ShowStandaloneAlert("انتهاء الاشتراك", "عزيزي انتهى الاشتراك مالتك راسل المطور\ntele : @zcr_c")
		return false
	}

	fmt.Printf("Subscription expires on: %s\n", subscriptionExpiry.Format("2006-01-02"))
	return true
}

func loadFiles() error {
	var err error
	users, err = utils.ReadFileLines("users.txt")
	if err != nil {
		return err
	}
	sessions, err = utils.ReadFileLines("sessions.txt")
	if err != nil {
		return err
	}
	proxies, err = utils.ReadFileLines("proxies.txt")
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		ui.ShowStandaloneAlert("تنبيه السيشنات", "ملف السيشنات فارغ، يرجى إضافة السيشنات ثم إعادة تشغيل البرنامج.")
		return fmt.Errorf("sessions.txt is empty")
	}
	if len(users) == 0 {
		ui.ShowStandaloneAlert("تنبيه اليوزرات", "ملف اليوزرات فارغ، يرجى إضافة اليوزرات ثم إعادة تشغيل البرنامج.")
	}
	return nil
}

func showSessionsEmptyAlert(dashboard *ui.Dashboard) {
	remaining, err := utils.ReadFileLines("sessions.txt")
	if err == nil && len(remaining) == 0 {
		sessionsEmptyAlerted.Do(func() {
			dashboard.ShowAlert("تنبيه السيشنات", "ملف السيشنات فارغ بعد حذف آخر سيشن، تم إيقاف البرنامج.")
			os.Exit(0)
		})
	}
}

func showUsersEmptyAlertAndStop(dashboard *ui.Dashboard) {
	dashboard.ShowAlert("تنبيه اليوزرات", "ملف اليوزرات فضى أثناء التشغيل، تم إيقاف البرنامج.")
	os.Exit(0)
}

func getRandomProxy() string {
	if len(proxies) == 0 {
		return ""
	}
	randomMu.Lock()
	idx := rng.Intn(len(proxies))
	randomMu.Unlock()
	return proxies[idx]
}

func randomString(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, n)

	randomMu.Lock()
	defer randomMu.Unlock()
	for i := range result {
		result[i] = chars[rng.Intn(len(chars))]
	}
	return string(result)
}

func fastHex(n int) string {
	result := make([]byte, n/2+1)
	if _, err := rand.Read(result); err != nil {
		return strings.Repeat("0", n)
	}
	return hex.EncodeToString(result)[:n]
}

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode = kernel32.NewProc("GetConsoleMode")
	setConsoleMode = kernel32.NewProc("SetConsoleMode")
)

func enableANSI() {
	handle := syscall.Handle(os.Stdout.Fd())
	var mode uint32
	ret, _, _ := getConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode)))
	if ret != 0 {
		_, _, _ = setConsoleMode.Call(uintptr(handle), uintptr(mode|0x0004))
	}
}

func printBanner() {
	enableANSI()
	fmt.Print("\x1b[34m")
	fmt.Println(` ███████████ █████                   █████████  █████       ███                         ███   █████   `)
	fmt.Println(`░█░░░███░░░█░░███                   ███░░░░░███░░███       ░░░                         ░░░   ░░███    `)
	fmt.Println(`░   ░███  ░  ░███████    ██████    ░███    ░░░  ░███████   ████   █████████ ████████   ████  ███████  `)
	fmt.Println(`    ░███     ░███░░███  ███░░███   ░░█████████  ░███░░███ ░░███  ░█░░░░███ ░░███░░███ ░░███ ░░░███░   `)
	fmt.Println(`    ░███     ░███ ░███ ░███████     ░░░░░░░░███ ░███ ░███  ░███  ░   ███░   ░███ ░███  ░███   ░███    `)
	fmt.Println(`    ░███     ░███ ░███ ░███░░░      ███    ░███ ░███ ░███  ░███    ███░   █ ░███ ░███  ░███   ░███ ███`)
	fmt.Println(`    █████    ████ █████░░██████    ░░█████████  ████ █████ █████  █████████ ████ █████ █████  ░░█████ `)
	fmt.Println(`   ░░░░░    ░░░░ ░░░░░  ░░░░░░      ░░░░░░░░░  ░░░░ ░░░░░ ░░░░░  ░░░░░░░░░ ░░░░ ░░░░░ ░░░░░    ░░░░░  `)
	fmt.Println(`_________________________`)
	fmt.Println(``)
	fmt.Println(`	The Shiznit v2`)
	fmt.Println(`_________________________`)
	fmt.Println(``)
	fmt.Print("\x1b[0m")
}

func checkTargetAndClaim(target string, dashboard *ui.Dashboard, claimQueue chan<- string, stats *insta.Stats) {
	stats.Checker.Lock.Lock()
	stats.Checker.Checked++
	stats.Checker.WindowChecked++
	stats.Checker.Lock.Unlock()

	fakeIGDID := fmt.Sprintf("%s-%s-%s-%s-%s", fastHex(8), fastHex(4), fastHex(4), fastHex(4), fastHex(12))
	fakeCSRF := "CSRFT-" + randomString(20)
	fakeLSD := randomString(11)
	randomEmail := randomString(8) + "@gmail.com"

	headers := map[string]string{
		"Host":               "www.instagram.com",
		"User-Agent":         "Mozilla/5.0",
		"X-Fb-Friendly-Name": "useCAARegistrationFieldValidationQuery",
		"Content-Type":       "application/x-www-form-urlencoded",
		"Connection":         "keep-alive",
		"x-csrftoken":        fakeCSRF,
		"x-fb-lsd":           fakeLSD,
		"Cookie":             "ig_did=" + strings.ToUpper(fakeIGDID),
	}

	variables := map[string]any{
		"input": map[string]any{
			"contactpoint":      map[string]string{"sensitive_string_value": randomEmail},
			"contactpoint_type": "EMAIL",
			"field_name":        "USERNAME",
			"username":          map[string]string{"sensitive_string_value": target},
		},
		"scale": 1,
	}
	variablesJSON, err := json.Marshal(variables)
	if err != nil {
		return
	}

	payload := url.Values{}
	payload.Set("lsd", fakeLSD)
	payload.Set("fb_api_req_friendly_name", "useCAARegistrationFieldValidationQuery")
	payload.Set("server_timestamps", "true")
	payload.Set("variables", string(variablesJSON))
	payload.Set("doc_id", "25391252800555418")

	client := insta.GlobalClient

	selectedProxy := getRandomProxy()
	if selectedProxy != "" {
		if !strings.HasPrefix(selectedProxy, "http://") && !strings.HasPrefix(selectedProxy, "https://") {
			selectedProxy = "http://" + selectedProxy
		}
		proxyURL, err := url.Parse(selectedProxy)
		if err == nil {
			client = &http.Client{
				Transport: &http.Transport{
					Proxy:                 http.ProxyURL(proxyURL),
					ForceAttemptHTTP2:     true,
					MaxIdleConns:          100,
					IdleConnTimeout:       90 * time.Second,
					TLSHandshakeTimeout:   10 * time.Second,
					ExpectContinueTimeout: 1 * time.Second,
				},
				Timeout: 10 * time.Second,
				CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
		}
	}

	req, err := http.NewRequest(http.MethodPost, "https://www.instagram.com/api/graphql", strings.NewReader(payload.Encode()))
	if err != nil {
		stats.Checker.Lock.Lock()
		stats.Checker.Errors++
		stats.Checker.Lock.Unlock()
		return
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		stats.Checker.Lock.Lock()
		stats.Checker.Errors++
		stats.Checker.Lock.Unlock()
		if resp != nil {
			resp.Body.Close()
		}
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		stats.Checker.Lock.Lock()
		stats.Checker.Errors++
		stats.Checker.Lock.Unlock()
		return
	}

	var data struct {
		Data struct {
			Validation struct {
				Status string `json:"status"`
			} `json:"xfb_caa_registration_field_validation"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		stats.Checker.Lock.Lock()
		stats.Checker.Errors++
		stats.Checker.Lock.Unlock()
		return
	}

	if data.Data.Validation.Status == "SUCCESS" {
		dashboard.Log(fmt.Sprintf("[Checker] [AVAILABLE] %s found!", target), "green")
		claimQueue <- target
	}
}

func prepareClients(count int, stats *insta.Stats, dashboard *ui.Dashboard) []*insta.InstaClient {
	if count < 1 {
		count = 1
	}
	if count > len(sessions) {
		count = len(sessions)
	}

	clients := make([]*insta.InstaClient, 0, count)
	for i := 0; i < count; i++ {
		client := insta.NewInstaClient(sessions[i], stats, getRandomProxy)
		if err := client.FetchSharedData(); err != nil {
			dashboard.Log(fmt.Sprintf("[Claimer] invalid session %d removed (CSRF setup failed): %v", i+1, err), "red")
			_ = utils.RemoveFromFile("sessions.txt", client.SessionID)
			showSessionsEmptyAlert(dashboard)

			continue
		}
		if err := client.ProfileInfo(); err != nil {
			dashboard.Log(fmt.Sprintf("[Claimer] invalid session %d removed (profile setup failed): %v", i+1, err), "red")
			_ = utils.RemoveFromFile("sessions.txt", client.SessionID)
			showSessionsEmptyAlert(dashboard)

			continue
		}
		clients = append(clients, client)
	}
	return clients
}

func main() {
	printBanner()

	if !checkSubscription() {
		return
	}

	if err := loadFiles(); err != nil {
		fmt.Println("Error:", err)
		return
	}

	var claimThreads int
	fmt.Print("Enter number of claimer threads: ")
	fmt.Scanln(&claimThreads)
	if claimThreads < 1 {
		claimThreads = 1
	}

	var useChecker string
	fmt.Print("Do you want to run the checker? (y/n): ")
	fmt.Scanln(&useChecker)

	var checkerThreads int
	if strings.ToLower(useChecker) == "y" || strings.ToLower(useChecker) == "yes" {
		fmt.Print("Enter number of checker threads: ")
		fmt.Scanln(&checkerThreads)
		if checkerThreads < 1 {
			checkerThreads = 1
		}
	}

	stats := &insta.Stats{}
	dashboard := ui.NewDashboard(stats)
	claimQueue := make(chan string, 1000)

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			trustedNow, err := getTrustedTime()
			if err != nil {
				dashboard.Log(fmt.Sprintf("[Subscription] Online time verification failed: %v. The tool has stopped.", err), "red")
				dashboard.App.Stop()
				return
			}
			if !trustedNow.Before(subscriptionExpiry.UTC()) {
				dashboard.Log(fmt.Sprintf("[Subscription] Expired on %s. The tool has stopped.", subscriptionExpiry.Format("2006-01-02")), "red")
				dashboard.ShowAlertAndStop("انتهاء الاشتراك", "عزيزي انتهى الاشتراك مالتك راسل المطور\ntele : @zcr_c")
				return
			}
		}
	}()

	sessionQueue := &lineCycler{name: "sessions.txt"}
	var cacheMu sync.Mutex
	clientCache := make(map[string]*insta.InstaClient)
	var wgClaimers sync.WaitGroup
	for i := 0; i < claimThreads; i++ {
		wgClaimers.Add(1)
		go func(id int) {
			defer wgClaimers.Done()
			for {
				target, err := nextClaimTarget(claimQueue)
				if err != nil {
					time.Sleep(10 * time.Millisecond)
					continue
				}

				// التحقق الذري المسبق
				if _, alreadyClaimed := claimedTargets.Load(target); alreadyClaimed {
					continue
				}

				session, err := sessionQueue.next()
				if err != nil {
					time.Sleep(10 * time.Millisecond)
					continue
				}

				cacheMu.Lock()
				client := clientCache[session]
				cacheMu.Unlock()
				if client == nil {
					client = insta.NewInstaClient(session, stats, getRandomProxy)
					if err := client.FetchSharedData(); err != nil {
						dashboard.Log(fmt.Sprintf("[Claimer] session setup failed: %v", err), "red")
						_ = utils.RemoveFromFile("sessions.txt", session)
						showSessionsEmptyAlert(dashboard)

						continue
					}
					if err := client.ProfileInfo(); err != nil {
						dashboard.Log(fmt.Sprintf("[Claimer] invalid session %s removed: %v", client.GetUserID(), err), "red")
						_ = utils.RemoveFromFile("sessions.txt", session)
						showSessionsEmptyAlert(dashboard)

						continue
					}
					cacheMu.Lock()
					clientCache[session] = client
					cacheMu.Unlock()
				}

				// إعادة فحص سريعة قبل إرسال الطلب للحماية
				if _, alreadyClaimed := claimedTargets.Load(target); alreadyClaimed {
					continue
				}

				success, err := client.ClaimUser(target)
				if err == nil && success {
					// فحص ذري سريع: أول ثريد يحصل على false هو الفائز الوحيد
					_, loaded := claimedTargets.LoadOrStore(target, true)
					if !loaded {
						dashboard.Log(fmt.Sprintf("Claimer Thread %d: [SUCCESS] Claimed %s", id, target), "green")
						dashboard.ShowAlert("تم الصيد بنجاح", fmt.Sprintf("   عاشت ايدك صدت : %s", target))
						client.RemoveSelf(true)
					}
				} else if err != nil {
					dashboard.Log(fmt.Sprintf("Claimer Thread %d: request failed: %v", id, err), "red")
				}
			}
		}(i + 1)
	}

	if strings.ToLower(useChecker) == "y" || strings.ToLower(useChecker) == "yes" {
		go func() {
			usersEmptyAlerted := false
			for {
				currentUsers, err := utils.ReadFileLines("users.txt")
				if err != nil || len(currentUsers) == 0 {
					if err == nil && !usersEmptyAlerted {
						showUsersEmptyAlertAndStop(dashboard)

						usersEmptyAlerted = true
					}
					time.Sleep(10 * time.Millisecond)
					continue
				}
				usersEmptyAlerted = false

				jobs := make(chan string, len(currentUsers))
				var wgChecker sync.WaitGroup
				for i := 0; i < checkerThreads; i++ {
					wgChecker.Add(1)
					go func() {
						defer wgChecker.Done()
						for username := range jobs {
							checkTargetAndClaim(username, dashboard, claimQueue, stats)
						}
					}()
				}
				for _, username := range currentUsers {
					jobs <- username
				}
				close(jobs)
				wgChecker.Wait()
			}
		}()
	} else {
		go func() {
			usersEmptyAlerted := false
			for {
				currentUsers, err := utils.ReadFileLines("users.txt")
				if err != nil || len(currentUsers) == 0 {
					if err == nil && !usersEmptyAlerted {
						showUsersEmptyAlertAndStop(dashboard)

						usersEmptyAlerted = true
					}
					time.Sleep(10 * time.Millisecond)
					continue
				}
				usersEmptyAlerted = false
				for _, username := range currentUsers {
					if _, alreadyClaimed := claimedTargets.Load(username); !alreadyClaimed {
						claimQueue <- username
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	if err := dashboard.Run(); err != nil {
		fmt.Printf("Error running dashboard: %v\n", err)
	}
}
