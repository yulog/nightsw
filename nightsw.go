//go:generate goversioninfo
package main

import (
	"context"
	"log"
	"time"

	"github.com/lxn/walk"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	appName string = "Night Switch"

	keyPath   string = `SOFTWARE\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	valueName string = "AppsUseLightTheme"
)

func main() {
	// Windows 10 以外なら終了
	ver := windows.RtlGetVersion()
	if ver.MajorVersion != uint32(10) {
		log.Fatal("incompatible version")
	}

	// メインウィンドウ作成
	mw, err := walk.NewMainWindow()
	if err != nil {
		log.Fatal(err)
	}

	// リソースからアイコンを読み込み
	icon, err := walk.NewIconFromResourceId(2)
	if err != nil {
		log.Fatal(err)
	}

	// 通知アイコンを作成
	ni, err := walk.NewNotifyIcon(mw)
	if err != nil {
		log.Fatal(err)
	}
	defer ni.Dispose()

	// アイコンとツールチップを設定
	if err := ni.SetIcon(icon); err != nil {
		log.Fatal(err)
	}
	if err := ni.SetToolTip(appName); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// マウスを放したときにイベント
	mouseUp := make(chan struct{})
	ni.MouseUp().Attach(func(x, y int, button walk.MouseButton) {
		if button != walk.LeftButton {
			return
		}
		mouseUp <- struct{}{}
	})

	// 短期間に届くトリガーを無視
	// 参考:GoCon2021Spring ホットリロードツールの作り方
	trg := make(chan struct{}, 1)
	go func() {
	mouse:
		for {
			<-mouseUp
			select {
			case trg <- struct{}{}:
			case <-ctx.Done():
				break mouse
			default:
			}
		}
	}()

	go func() {
	update:
		for {
			select {
			case <-trg:
			case <-ctx.Done():
				break update
			default:
			}
			<-trg
			err = update(ni, icon)
			if err != nil {
				log.Fatal(err)
			}
			<-time.NewTimer(time.Second * 5).C
		}
	}()

	// コンテキストメニューに終了を設定
	exitAction := walk.NewAction()
	if err := exitAction.SetText("終了"); err != nil {
		log.Fatal(err)
	}
	exitAction.Triggered().Attach(func() { walk.App().Exit(0) })
	if err := ni.ContextMenu().Actions().Add(exitAction); err != nil {
		log.Fatal(err)
	}

	// 通知アイコンを可視化
	if err := ni.SetVisible(true); err != nil {
		log.Fatal(err)
	}

	// ループ
	mw.Run()
}

func update(ni *walk.NotifyIcon, icon *walk.Icon) error {
	// HKEY_CURRENT_USERのレジストリを変更する
	// レジストリキーの作成、値の設定の権限
	k, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	// 設定値を取得、なかったらデフォルト設定
	_, _, err = k.GetIntegerValue(valueName)
	if err != nil {
		err = k.SetDWordValue(valueName, uint32(1))
		if err != nil {
			return err
		}
	}

	// 設定値を再取得
	v, _, err := k.GetIntegerValue(valueName)
	if err != nil {
		return err
	}

	// テーマを切り替える
	var theme uint32
	switch v {
	case uint64(1):
		theme = 0
	case uint64(0):
		theme = 1
	}
	err = k.SetDWordValue(valueName, theme)
	if err != nil {
		if err := ni.ShowError(appName, "既定のアプリモードの変更に失敗しました。"); err != nil {
			return err
		}
		return err
	}

	// 完了の通知
	if err := ni.ShowCustom(
		appName,
		"既定のアプリモードを変更しました。",
		icon); err != nil {
		return err
	}
	return nil
}
