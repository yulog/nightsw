//go:generate goversioninfo
package main

import (
	"context"
	"log"
	"time"

	"github.com/lxn/walk"
	"golang.org/x/sys/windows/registry"
)

const appName = "Night Switch"

const valueName = "AppsUseLightTheme"

func main() {
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
	// defer func() { log.Println("ctx"); cancel() }()

	// マウスを放したときにイベント
	mouseUp := make(chan struct{})
	//defer func() { log.Println("mouseUp"); close(mouseUp) }()
	ni.MouseUp().Attach(func(x, y int, button walk.MouseButton) {
		if button != walk.LeftButton {
			return
		}
		mouseUp <- struct{}{}
	})

	// 短期間に届くトリガーを無視
	// 参考:GoCon2021Spring ホットリロードツールの作り方
	trg := make(chan struct{}, 1)
	//defer func() { log.Println("trg"); close(trg) }()
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
			log.Println("Click")
			err = update(ni, icon)
			if err != nil {
				//log.Println(err)
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
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	//defer func() { log.Println("regist"); k.Close() }()

	// 設定値を取得、なかったらデフォルト設定
	v, _, err := k.GetIntegerValue(valueName)
	log.Println(v)
	if err != nil {
		//log.Println(err)
		err = k.SetDWordValue(valueName, uint32(1))
		if err != nil {
			return err
		}
	}

	// 設定値、再取得
	v, _, err = k.GetIntegerValue(valueName)
	//log.Println(v)
	if err != nil {
		return err
	}

	// テーマを切り替える
	var theme uint32
	switch v {
	case uint64(1):
		//log.Println("light to dark")
		theme = 0
	case uint64(0):
		//log.Println("dark to light")
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
