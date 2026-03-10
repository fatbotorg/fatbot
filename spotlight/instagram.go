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

// UserRequestedSpotlight is triggered when a user replies "insta [caption]" to
// their own workout photo in a group chat. It uses the specific photo from the
// replied-to message rather than the user's last workout photo.
func UserRequestedSpotlight(bot *tgbotapi.BotAPI, user users.User, photoFileID string, chatId int64, customCaption string) {
	log.Infof("User-requested Instagram spotlight: %s (chatId: %d)", user.GetName(), chatId)
	CreateInstagramStoryFromPhoto(bot, user, photoFileID, chatId, customCaption)
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

// CreateInstagramStory fetches the user's last workout photo and publishes it.
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
	CreateInstagramStoryFromPhoto(bot, user, lastWorkout.PhotoFileID, chatId, "")
}

// CreateInstagramStoryFromPhoto is the core pipeline: given an explicit photoFileID,
// it renders branded images and publishes them to Instagram.
// customCaption is appended to the generated Instagram caption when non-empty.
func CreateInstagramStoryFromPhoto(bot *tgbotapi.BotAPI, user users.User, photoFileID string, chatId int64, customCaption string) {
	log.Debug("Starting Instagram story creation", "user", user.GetName(), "chatId", chatId, "fileId", photoFileID)

	// 1. Download image
	log.Debug("Downloading photo from Telegram", "fileId", photoFileID)
	fileConfig := tgbotapi.FileConfig{FileID: photoFileID}
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
	if customCaption != "" {
		caption = fmt.Sprintf("%s\n\n💬 %s", caption, customCaption)
	}

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

// fontPaths returns candidate font paths in preference order:
// bundled Montserrat first, then system fallbacks for Alpine/Debian/macOS.
func resolveFontPath(variant string) string {
	// variant: "extrabold", "bold", "medium"
	bundled := map[string]string{
		"extrabold": "spotlight/fonts/Montserrat-ExtraBold.ttf",
		"bold":      "spotlight/fonts/Montserrat-Bold.ttf",
		"medium":    "spotlight/fonts/Montserrat-Medium.ttf",
	}
	systemFallbacks := []string{
		"/usr/share/fonts/freefont/FreeSansBold.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
		"/usr/share/fonts/ttf-dejavu/DejaVuSans-Bold.ttf",
		"/usr/share/fonts/dejavu/DejaVuSans-Bold.ttf",
		"/System/Library/Fonts/Supplemental/DIN Alternate Bold.ttf",
		"/System/Library/Fonts/Supplemental/Arial Bold.ttf",
	}
	if p, ok := bundled[variant]; ok {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, p := range systemFallbacks {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// drawStrokedText draws white text with a black outline for legibility on any background.
func drawStrokedText(dc *gg.Context, x, y float64, text string, strokePx int, alpha float64) {
	// Black outline — draw in a grid around the origin
	dc.SetRGBA(0, 0, 0, 0.85)
	s := float64(strokePx)
	for dx := -s; dx <= s; dx += 2 {
		for dy := -s; dy <= s; dy += 2 {
			if dx == 0 && dy == 0 {
				continue
			}
			dc.DrawStringAnchored(text, x+dx, y+dy, 0.5, 0.5)
		}
	}
	// White text on top
	dc.SetRGBA(1, 1, 1, alpha)
	dc.DrawStringAnchored(text, x, y, 0.5, 0.5)
}

// drawBottomGradient draws a smooth gradient from transparent to near-black
// over the bottom portion of the image.
func drawBottomGradient(dc *gg.Context, width, height int, startPct float64) {
	w := float64(width)
	h := float64(height)
	startY := h * startPct
	steps := 300
	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps-1)
		y := startY + t*(h-startY)
		// Ease-in curve: slow start, strong finish
		alpha := t * t * 0.88
		dc.SetRGBA(0, 0, 0, alpha)
		dc.DrawRectangle(0, y, w, (h-startY)/float64(steps)+1)
		dc.Fill()
	}
}

// drawTopVignette draws a very subtle top darkening for polish.
func drawTopVignette(dc *gg.Context, width, height int) {
	vigH := float64(height) * 0.14
	for i := 0; i < 60; i++ {
		t := float64(i) / 60.0
		y := t * vigH
		alpha := (1.0 - t) * (1.0 - t) * 0.28
		dc.SetRGBA(0, 0, 0, alpha)
		dc.DrawRectangle(0, y, float64(width), vigH/60+1)
		dc.Fill()
	}
}

// drawCornerBrackets draws subtle corner bracket accents.
func drawCornerBrackets(dc *gg.Context, width, height int) {
	w := float64(width)
	h := float64(height)
	m := 35.0 // margin
	l := 55.0 // bracket arm length
	lw := 3.0 // line width

	dc.SetRGBA(1, 1, 1, 0.32)
	dc.SetLineWidth(lw)

	// Top-left
	dc.DrawLine(m, m, m+l, m)
	dc.Stroke()
	dc.DrawLine(m, m, m, m+l)
	dc.Stroke()
	// Top-right
	dc.DrawLine(w-m, m, w-m-l, m)
	dc.Stroke()
	dc.DrawLine(w-m, m, w-m, m+l)
	dc.Stroke()
	// Bottom-left
	dc.DrawLine(m, h-m, m+l, h-m)
	dc.Stroke()
	dc.DrawLine(m, h-m, m, h-m-l)
	dc.Stroke()
	// Bottom-right
	dc.DrawLine(w-m, h-m, w-m-l, h-m)
	dc.Stroke()
	dc.DrawLine(w-m, h-m, w-m, h-m-l)
	dc.Stroke()
}

func renderHighImpactImage(src image.Image, width, height int, name, title string, count int, outFile string) error {
	log.Debug("Rendering image", "width", width, "height", height, "name", name, "count", count)
	dc := gg.NewContext(width, height)
	w := float64(width)
	h := float64(height)
	scale := w / 1080.0

	// ── A. Background: cover-crop with top bias (show faces, not feet) ──
	srcW, srcH := src.Bounds().Dx(), src.Bounds().Dy()
	srcRatio := float64(srcW) / float64(srcH)
	destRatio := w / h

	var srcRect image.Rectangle
	if srcRatio > destRatio {
		newSrcW := int(float64(srcH) * destRatio)
		offset := (srcW - newSrcW) / 2
		srcRect = image.Rect(offset, 0, offset+newSrcW, srcH)
	} else {
		newSrcH := int(float64(srcW) / destRatio)
		// Bias toward top 20% so faces appear rather than feet
		offset := int(float64(srcH-newSrcH) * 0.2)
		srcRect = image.Rect(0, offset, srcW, offset+newSrcH)
	}

	if rgba, ok := dc.Image().(*image.RGBA); ok {
		xdraw.CatmullRom.Scale(rgba, rgba.Bounds(), src, srcRect, xdraw.Over, nil)
	} else {
		scaleW := w / float64(srcRect.Dx())
		scaleH := h / float64(srcRect.Dy())
		dc.Scale(scaleW, scaleH)
		dc.DrawImage(src, -srcRect.Min.X, -srcRect.Min.Y)
		dc.Identity()
	}

	// ── B. Overlays ──
	drawBottomGradient(dc, width, height, 0.40)
	drawTopVignette(dc, width, height)
	drawCornerBrackets(dc, width, height)

	// ── C. Resolve fonts ──
	fontEB := resolveFontPath("extrabold")
	fontMed := resolveFontPath("medium")
	fontBold := resolveFontPath("bold")
	if fontEB == "" {
		fontEB = fontBold
	}
	if fontMed == "" {
		fontMed = fontBold
	}
	if fontEB == "" {
		log.Warn("No suitable font found for image rendering")
	}

	cx := w / 2

	// ── D. Motivational title — top, large and bold ──
	if fontEB != "" {
		if err := dc.LoadFontFace(fontEB, 90*scale); err != nil {
			log.Warnf("Failed to load font: %s", err)
		}
	}
	drawStrokedText(dc, cx, h*0.07, title, 5, 1.0)

	// ── E. Workout count — the hero number ──
	countY := h * 0.65
	if fontEB != "" {
		if err := dc.LoadFontFace(fontEB, 340*scale); err != nil {
			log.Warnf("Failed to load font: %s", err)
		}
	}
	drawStrokedText(dc, cx, countY, fmt.Sprintf("%d", count), 8, 1.0)

	// ── F. "WORKOUTS THIS WEEK" label ──
	labelY := countY + 145*scale
	if fontMed != "" {
		if err := dc.LoadFontFace(fontMed, 52*scale); err != nil {
			log.Warnf("Failed to load font: %s", err)
		}
	}
	drawStrokedText(dc, cx, labelY, "WORKOUTS THIS WEEK", 4, 0.95)

	// ── G. Thin separator line ──
	lineY := labelY + 55*scale
	lineHalf := w * 0.15
	dc.SetRGBA(1, 1, 1, 0.45)
	dc.SetLineWidth(2)
	dc.DrawLine(cx-lineHalf, lineY, cx+lineHalf, lineY)
	dc.Stroke()

	// ── H. Instagram handle ──
	if fontBold != "" {
		if err := dc.LoadFontFace(fontBold, 60*scale); err != nil {
			log.Warnf("Failed to load font: %s", err)
		}
	}
	drawStrokedText(dc, cx, h*0.90, name, 5, 1.0)

	// ── I. Branding ──
	if fontMed != "" {
		if err := dc.LoadFontFace(fontMed, 30*scale); err != nil {
			log.Warnf("Failed to load font: %s", err)
		}
	}
	drawStrokedText(dc, cx, h*0.945, "fatbot.fit", 3, 0.65)

	// ── Encode ──
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dc.Image(), &jpeg.Options{Quality: 95}); err != nil {
		return err
	}
	log.Debug("Successfully encoded image to JPEG", "outFile", outFile)
	return os.WriteFile(outFile, buf.Bytes(), 0644)
}
