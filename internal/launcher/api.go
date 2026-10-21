package launcher

import (
	"context"
	"encoding/json"
	"errors"
)

type command struct {
	RootID        string   `json:"rootId"`
	AccountID     string   `json:"accountId"`
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Path          string   `json:"path"`
	Parent        string   `json:"parent"`
	Paths         []string `json:"paths"`
	Version       string   `json:"version"`
	Minecraft     string   `json:"minecraft"`
	Loader        string   `json:"loader"`
	LoaderVersion string   `json:"loaderVersion"`
	JavaPath      string   `json:"javaPath"`
	MemoryMB      int      `json:"memoryMB"`
	JVMArgs       string   `json:"jvmArgs"`
	Source        string   `json:"source"`
	ProjectID     string   `json:"projectId"`
	VersionID     string   `json:"versionId"`
	Replace       string   `json:"replace"`
	Server        string   `json:"server"`
	Username      string   `json:"username"`
	Password      string   `json:"password"`
	ProfileID     string   `json:"profileId"`
	Settings      Settings `json:"settings"`
	Kind          string   `json:"kind"`
	Model         string   `json:"model"`
	Data          string   `json:"data"`
	CapeID        string   `json:"capeId"`
}

// Dispatch connects native UI actions to the launcher service. It never returns account tokens.
func (s *Service) Dispatch(ctx context.Context, method, payload string) (string, error) {
	if len(payload) > 8<<20 {
		return "", errors.New("请求过大")
	}
	var c command
	if payload != "" {
		if e := json.Unmarshal([]byte(payload), &c); e != nil {
			return "", e
		}
	}
	var value any
	var err error
	switch method {
	case "snapshot":
		value = s.Snapshot()
	case "addRoot":
		value, err = s.AddRoot(c.Path, c.Name)
	case "removeRoot":
		err = s.RemoveRoot(c.RootID)
	case "updateRoot":
		err = s.UpdateRoot(c.RootID, c.Name, c.JavaPath, c.MemoryMB, c.JVMArgs)
	case "updateVersion":
		err = s.UpdateVersionConfig(c.RootID, c.Version, c.JavaPath, c.MemoryMB, c.JVMArgs)
	case "setVersionName":
		err = s.SetVersionDisplayName(c.RootID, c.Version, c.Name)
	case "select":
		err = s.Select(c.RootID, c.AccountID, c.Version)
	case "settings":
		err = s.SaveSettings(c.Settings)
	case "mods":
		value, err = s.ListMods(c.RootID)
	case "toggleMod":
		err = s.ToggleMod(c.RootID, c.Name)
	case "removeMod":
		err = s.RemoveMod(c.RootID, c.Name)
	case "addLocalMods":
		err = s.AddLocalMods(c.RootID, c.Paths)
	case "checkMods":
		value, err = s.CheckMods(c.RootID)
	case "installMod":
		value, err = s.InstallMod(c.RootID, c.Source, c.ProjectID, c.VersionID, c.Replace)
	case "assetPacks":
		value, err = s.ListAssetPacks(c.RootID, c.Kind)
	case "toggleAssetPack":
		err = s.ToggleAssetPack(c.RootID, c.Kind, c.Name)
	case "removeAssetPack":
		err = s.RemoveAssetPack(c.RootID, c.Kind, c.Name)
	case "addLocalAssetPacks":
		err = s.AddLocalAssetPacks(c.RootID, c.Kind, c.Paths)
	case "installAssetPack":
		value, err = s.InstallAssetPack(c.RootID, c.Kind, c.Source, c.ProjectID, c.VersionID)
	case "search":
		var q SearchQuery
		if err = json.Unmarshal([]byte(payload), &q); err == nil {
			value, err = s.Search(ctx, q)
		}
	case "versions":
		value, err = s.Versions(ctx, c.Source, c.ProjectID, c.Minecraft, c.Loader)
	case "inspectPack":
		value, err = s.InspectPack(c.Path)
	case "importPack":
		var q ImportRequest
		if err = json.Unmarshal([]byte(payload), &q); err == nil {
			value, err = s.ImportPack(q)
		}
	case "checkPackUpdate":
		value, err = s.CheckPackUpdate(ctx, c.RootID)
	case "planPackUpdate":
		value, err = s.PlanPackUpdate(ctx, c.RootID, c.Path, c.VersionID)
	case "applyPackUpdate":
		value, err = s.ApplyPackUpdate(c.ID)
	case "discardPackUpdate":
		s.DiscardPackUpdate(c.ID)
	case "rollbackPack":
		value, err = s.RollbackPack(c.RootID, c.ID)
	case "packUpdateReport":
		value, err = s.PackUpdateReport(c.RootID, c.ID)
	case "checkPack":
		value, err = s.CheckPack(ctx, c.RootID)
	case "manifest":
		value, err = s.Manifest(ctx)
	case "loaderVersions":
		value, err = s.LoaderVersions(ctx, c.Loader, c.Minecraft)
	case "installGame":
		value, err = s.InstallGame(c.RootID, c.Parent, c.Name, c.Minecraft, c.Loader, c.LoaderVersion)
	case "javaList":
		value = s.JavaList(ctx)
	case "systemMemory":
		value, err = SystemMemory()
	case "launch":
		value, err = s.Launch(c.RootID, c.AccountID)
	case "stop":
		err = s.Stop(c.RootID)
	case "logs":
		value, err = s.Logs(c.RootID)
	case "cancelJob":
		err = s.cancelJob(c.ID)
	case "offlineLogin":
		value, err = s.AddOffline(c.Name)
	case "startMicrosoft":
		value, err = s.StartMicrosoft(ctx)
	case "pollMicrosoft":
		value, err = s.PollMicrosoft(ctx, c.ID)
	case "cancelMicrosoft":
		s.CancelMicrosoft(c.ID)
	case "thirdLogin":
		value, err = s.LoginThird(ctx, c.Server, c.Username, c.Password, c.ProfileID)
	case "saveAuthServer":
		value, err = s.SaveAuthServer(c.ID, c.Name, c.Server)
	case "deleteAuthServer":
		err = s.DeleteAuthServer(c.Server)
	case "fetchAuthServerName":
		value, err = s.FetchAuthServerName(ctx, c.Server)
	case "removeAccount":
		err = s.RemoveAccount(c.AccountID)
	case "appearance":
		value, err = s.Appearance(ctx, c.AccountID)
	case "uploadTexture":
		err = s.UploadTexture(ctx, c.AccountID, c.Kind, c.Model, c.Data)
	case "resetTexture":
		err = s.ResetTexture(ctx, c.AccountID, c.Kind)
	case "setCape":
		err = s.SetCape(ctx, c.AccountID, c.CapeID)
	default:
		return "", errors.New("未知操作")
	}
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(value)
	return string(b), err
}
func (s *Service) RootPath(id string) (string, error) {
	r, e := s.getRoot(id)
	return r.VersionDir(), e
}
