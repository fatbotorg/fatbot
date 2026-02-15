package spotlight

import (
	"bytes"
	"fatbot/ai"
	"fatbot/db"
	"fatbot/instagram"
	"fatbot/users"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/fogleman/gg"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	xdraw "golang.org/x/image/draw"
)

func DailyInstagramAutomation(bot *tgbotapi.BotAPI) {
	log.Info("Starting Daily Instagram Automation")
	var usersList []users.User
	// Join with workouts to ensure we only pick users who have at least one workout with a photo
	err := db.DBCon.Model(&users.User{}).
		Joins("JOIN workouts ON workouts.user_id = users.id").
		Where("users.instagram_handle != ? AND workouts.photo_file_id != ? AND workouts.photo_file_id != ?", "", "", "NULL").
		Group("users.id").
		Find(&usersList).Error
	if err != nil {
		log.Errorf("Error fetching instagram users with photos: %s", err)
		return
	}

	log.Infof("Found %d users enrolled for Instagram with workout photos", len(usersList))

	if len(usersList) == 0 {
		log.Info("No enrolled users with workout photos available for Instagram")
		return
	}

	// Pick a random user from the filtered list
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)
	randomUser := usersList[r.Intn(len(usersList))]

	chatIds, err := randomUser.GetChatIds()
	if err != nil || len(chatIds) == 0 {
		log.Warnf("Randomly selected user %s has no groups", randomUser.GetName())
		return
	}

	log.Infof("Daily Instagram selection: %s (ChatID: %d)", randomUser.GetName(), chatIds[0])
	CreateInstagramStory(bot, randomUser, chatIds[0])
}

func ManualInstagramSpotlight(bot *tgbotapi.BotAPI, user users.User) {
	chatIds, err := user.GetChatIds()
	if err != nil || len(chatIds) == 0 {
		log.Warnf("Manual selection user %s has no groups", user.GetName())
		return
	}
	log.Infof("Manual Instagram selection: %s", user.GetName())
	CreateInstagramStory(bot, user, chatIds[0])
}

func getPraiseMessage(count int) string {
	if count >= 5 {
		return "ABSOLUTE BEAST"
	}
	if count >= 3 {
		return "WORKOUT MONSTER"
	}
	return "PURE ELITE"
}

func CreateInstagramStory(bot *tgbotapi.BotAPI, user users.User, chatId int64) {
	log.Debug("Starting Instagram story creation", "user", user.GetName(), "chatId", chatId)
	lastWorkout, err := user.GetLastXWorkout(1, chatId)
	if err != nil {
		log.Errorf("Error getting last workout for instagram: %s", err)
		return
	}

	if lastWorkout.PhotoFileID == "" {
		log.Warn("User has no photo for their last workout, skipping Instagram story", "user", user.GetName())
		return
	}

	// 1. Download image
	log.Debug("Downloading photo from Telegram", "fileId", lastWorkout.PhotoFileID)
	fileConfig := tgbotapi.FileConfig{FileID: lastWorkout.PhotoFileID}
	tgFile, err := bot.GetFile(fileConfig)
	if err != nil {
		log.Errorf("Error getting file from Telegram: %s", err)
		return
	}

	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s",
		os.Getenv("TELEGRAM_APITOKEN"),
		tgFile.FilePath,
	)
	resp, err := http.Get(url)
	if err != nil {
		log.Errorf("Error downloading image: %s", err)
		return
	}
	defer resp.Body.Close()

	srcImg, _, err := image.Decode(resp.Body)
	if err != nil {
		log.Errorf("Error decoding workout image: %s", err)
		return
	}

	// 2. Prepare Data
	user.LoadWorkoutsThisCycle(chatId)
	workoutCount := len(user.Workouts)
	praise := getPraiseMessage(workoutCount)
	title := ai.GetAiMotivationalTitle()
	log.Debug("Prepared story data", "workoutCount", workoutCount, "title", title)

	// 3. Generate Visuals
	ts := time.Now().Unix()
	storyFile := fmt.Sprintf("story_%d_%d.jpg", user.TelegramUserID, ts)
	displayName := user.GetName()
	if user.InstagramHandle != "" {
		displayName = "@" + user.InstagramHandle
	}
	log.Debug("Rendering high impact story image", "outFile", storyFile, "handle", displayName)
	if err := renderHighImpactImage(srcImg, 1080, 1920, displayName, title, workoutCount, storyFile); err != nil {
		log.Errorf("Error rendering story: %s", err)
		return
	}

	postFile := fmt.Sprintf("post_%d_%d.jpg", user.TelegramUserID, ts)
	log.Debug("Rendering high impact post image", "outFile", postFile)
	if err := renderHighImpactImage(srcImg, 1080, 1080, user.GetName(), title, workoutCount, postFile); err != nil {
		log.Errorf("Error rendering post: %s", err)
		return
	}

	// 4. Upload & Post
	caption := fmt.Sprintf(`🦾 @%s is %s! 

They've crushed %d workouts this week. 
This is what consistency looks like. Keep pushing.

#FatBot #StayHard #NoDaysOff #FitnessMotivation #Unstoppable`,
		user.InstagramHandle, title, workoutCount)

	var storyID, postID string
	var pubErr error

	// Story
	storyBytes, err := os.ReadFile(storyFile)
	if err == nil && len(storyBytes) > 0 {
		log.Debug("Uploading story to S3")
		if publicStoryURL, err := users.UploadToS3(storyFile, storyBytes); err == nil {
			log.Infof("Story public URL: %s", publicStoryURL)
			storyCaption := fmt.Sprintf("@%s", user.InstagramHandle)
			log.Debug("Publishing story to Instagram")
			if storyID, pubErr = instagram.PublishStory(publicStoryURL, storyCaption); pubErr != nil {
				log.Errorf("Failed to publish story for %s: %s", user.GetName(), pubErr)
			} else {
				log.Infof("Successfully published story: %s", storyID)
			}
		} else {
			log.Errorf("Failed to upload story to S3: %s", err)
		}
	} else {
		log.Errorf("Failed to read story file or file is empty: %s (len: %d)", err, len(storyBytes))
	}

	// Post
	postBytes, err := os.ReadFile(postFile)
	if err == nil && len(postBytes) > 0 {
		log.Debug("Uploading post to S3")
		if publicPostURL, err := users.UploadToS3(postFile, postBytes); err == nil {
			log.Infof("Post public URL: %s", publicPostURL)
			log.Debug("Publishing post to Instagram")
			if postID, pubErr = instagram.PublishPost(publicPostURL, caption); pubErr != nil {
				log.Errorf("Failed to publish post for %s: %s", user.GetName(), pubErr)
			} else {
				log.Infof("Successfully published post: %s", postID)
			}
		} else {
			log.Errorf("Failed to upload post to S3: %s", err)
		}
	} else {
		log.Errorf("Failed to read post file or file is empty: %s (len: %d)", err, len(postBytes))
	}

	// 5. Consolidated Telegram Notification
	if storyID != "" || postID != "" {
		tgCaption := fmt.Sprintf(`🚀 BOOM! You've been featured on FatBot Instagram!

Handle: @%s
Status: %s
Workouts: %d

Check out the central account to see your spotlight! 💪 
https://www.instagram.com/fatbot.fit`,
			user.InstagramHandle, praise, workoutCount)

		log.Debug("Sending Telegram notification to user")
		// Send only the Post image as a confirmation preview
		msg := tgbotapi.NewPhoto(user.TelegramUserID, tgbotapi.FilePath(postFile))
		msg.Caption = tgCaption
		if _, err := bot.Send(msg); err != nil {
			log.Errorf("Error sending TG notification: %s", err)
		}
	} else {
		log.Errorf("Nothing was published to Instagram for user %s", user.GetName())
	}

	os.Remove(storyFile)
	os.Remove(postFile)
}

func renderHighImpactImage(src image.Image, width, height int, name, title string, count int, outFile string) error {
	log.Debug("Rendering image", "width", width, "height", height, "name", name, "count", count)
	dc := gg.NewContext(width, height)

	// A. Background (Cover)
	srcW, srcH := src.Bounds().Dx(), src.Bounds().Dy()
	srcRatio := float64(srcW) / float64(srcH)
	destRatio := float64(width) / float64(height)

	var srcRect image.Rectangle
	if srcRatio > destRatio {
		newSrcW := int(float64(srcH) * destRatio)
		offset := (srcW - newSrcW) / 2
		srcRect = image.Rect(offset, 0, offset+newSrcW, srcH)
	} else {
		newSrcH := int(float64(srcW) / destRatio)
		offset := (srcH - newSrcH) / 2
		srcRect = image.Rect(0, offset, srcW, offset+newSrcH)
	}

	if rgba, ok := dc.Image().(*image.RGBA); ok {
		xdraw.CatmullRom.Scale(rgba, rgba.Bounds(), src, srcRect, xdraw.Over, nil)
	} else {
		// Fallback if not RGBA
		log.Debug("Context image is not RGBA, using fallback scaling")
		scaleW := float64(width) / float64(srcRect.Dx())
		scaleH := float64(height) / float64(srcRect.Dy())
		dc.Scale(scaleW, scaleH)
		dc.DrawImage(src, -srcRect.Min.X, -srcRect.Min.Y)
		dc.Identity()
	}

	// B. Strong Modern Overlay
	dc.SetRGBA(0, 0, 0, 0.4)
	dc.DrawRectangle(0, 0, float64(width), float64(height))
	dc.Fill()

	// C. Typography Setup
	fontPaths := []string{
		"/usr/share/fonts/freefont/FreeSansBold.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
		"/usr/share/fonts/ttf-dejavu/DejaVuSans-Bold.ttf",
		"/usr/share/fonts/dejavu/DejaVuSans-Bold.ttf",
		"/System/Library/Fonts/Supplemental/DIN Alternate Bold.ttf",
		"/System/Library/Fonts/Supplemental/Arial Bold.ttf",
	}
	var fontPath string
	for _, path := range fontPaths {
		if _, err := os.Stat(path); err == nil {
			fontPath = path
			log.Debug("Found font", "path", fontPath)
			break
		}
	}
	if fontPath == "" {
		log.Warn("No suitable font found for image rendering - using default")
	}

	centerX := float64(width) / 2
	centerY := float64(height) / 2

	// D. BIG TOP TEXT: THE TITLE
	if fontPath != "" {
		if err := dc.LoadFontFace(fontPath, 100); err != nil {
			log.Warnf("Failed to load font %s: %s", fontPath, err)
		}
	}
	dc.SetRGB(1, 1, 1)
	dc.DrawStringAnchored(title, centerX, float64(height)*0.15, 0.5, 0.5)

	// E. SMALLER CENTER PROGRESS
	badgeRadius := float64(width) * 0.22
	dc.DrawCircle(centerX, centerY, badgeRadius)
	dc.SetRGBA(0.7, 1, 0, 0.8) // Neon Lime Green
	dc.Fill()

	if fontPath != "" {
		if err := dc.LoadFontFace(fontPath, 200); err != nil {
			log.Warnf("Failed to load font %s: %s", fontPath, err)
		}
	}
	dc.SetRGB(0, 0, 0)
	dc.DrawStringAnchored(fmt.Sprintf("%d", count), centerX, centerY, 0.5, 0.5)
	if fontPath != "" {
		if err := dc.LoadFontFace(fontPath, 28); err != nil {
			log.Warnf("Failed to load font %s: %s", fontPath, err)
		}
	}
	dc.DrawStringAnchored("WORKOUTS THIS WEEK", centerX, centerY+140, 0.5, 0.5)

	// F. HUGE BOTTOM NAME
	if fontPath != "" {
		if err := dc.LoadFontFace(fontPath, 120); err != nil {
			log.Warnf("Failed to load font %s: %s", fontPath, err)
		}
	}
	dc.SetRGBA(1, 1, 1, 1)
	dc.DrawStringAnchored(strings.ToUpper(name), centerX, float64(height)*0.85, 0.5, 0.5)

	// G. BOTTOM ACCENT
	dc.SetRGBA(0.7, 1, 0, 1)
	dc.DrawRectangle(centerX-300, float64(height)*0.92, 600, 20)
	dc.Fill()

	// H. BRANDING
	if fontPath != "" {
		if err := dc.LoadFontFace(fontPath, 30); err != nil {
			log.Warnf("Failed to load font %s: %s", fontPath, err)
		}
	}
	dc.SetRGBA(1, 1, 1, 0.5)
	dc.DrawStringAnchored("www.fatbot.fit", centerX, float64(height)-50, 0.5, 0.5)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dc.Image(), &jpeg.Options{Quality: 95}); err != nil {
		return err
	}
	log.Debug("Successfully encoded image to JPEG", "outFile", outFile)
	return os.WriteFile(outFile, buf.Bytes(), 0644)
}
