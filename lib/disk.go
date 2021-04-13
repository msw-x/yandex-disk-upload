package yandisk

import (
	"encoding/json"
	"fmt"
	"msw/moon"
	"net/http"
	"os"
)

// ссылка на методы яндекс диск api
// https://tech.yandex.com/disk/doc/dg/concepts/api-methods-docpage/

const ApiUrl = "https://cloud-api.yandex.net/v1/disk"

type Resource struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Created    string `json:"created"`
	ResourceId string `json:"resource_id"`
	Type       string `json:"type"`
	MimeType   string `json:"mime_type"`
	Embedded   struct {
		Items []Resource `json:"items"`
		Path  string     `json:"path"`
	} `json:"_embedded"`
}

// SystemFolders is the absolute addresses of Disk system folders.
type SystemFolders struct {
	Applications string `json:"applications"`
	Downloads    string `json:"downloads"`
}

// Disk represents the data about free and used space on the Disk.
// https://tech.yandex.com/disk/api/reference/response-objects-docpage/#disk
type Disk struct {
	// The cumulative size of the files in the Trash, in bytes.
	TrashSize uint `json:"trash_size"`

	// The total Disk space available to the user, in bytes.
	TotalSpace uint `json:"total_space"`

	// The cumulative size of the files already stored on the Disk, in bytes.
	UsedSpace uint `json:"used_space"`

	// Absolute addresses of Disk system folders.
	// Folder names depend on the user's interface language when the
	// personal Disk is created. For example, the "Downloads" folder
	// is created for an English-speaking user, the "Загрузки" folder
	// is created for a Russian-speaking user, and so on.
	//
	// The following folders are currently supported:
	//
	// * applications — folder for application files
	// * downloads — folder for files downloaded from
	// the internet (not from the user's device)
	SystemFolders SystemFolders `json:"system_folders"`
}

func TokenUrl(appID string) string {
	return fmt.Sprintf("https://oauth.yandex.ru/authorize?response_type=token&client_id=%s", appID)
}

type Client struct {
	apiUrl    string
	authToken string
}

func NewClient(authToken string) *Client {
	return &Client{
		apiUrl:    ApiUrl,
		authToken: authToken,
	}
}

func (this *Client) apiRequest(path, method string) (*http.Response, error) {
	client := http.Client{}
	url := fmt.Sprintf("%s/%s", this.apiUrl, path)
	req, _ := http.NewRequest(method, url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("OAuth %s", this.authToken))
	resp, err := client.Do(req)
	if err == nil {
		err = checkResponse(resp)
	}
	return resp, err
}

func (this *Client) GetDisk() (Disk, error) {
	res, err := this.apiRequest("", "GET")
	var disk Disk
	err = json.NewDecoder(res.Body).Decode(&disk)
	return disk, err
}

func (this *Client) GetDiskMust() Disk {
	disk, err := this.GetDisk()
	moon.Check(err)
	return disk
}

func (this *Client) CreateFolder(path string) error {
	_, err := this.apiRequest(fmt.Sprintf("resources?path=%s", path), "PUT")
	return err
}

func (this *Client) CreateFolderMust(path string) {
	err := this.CreateFolder(path)
	moon.Check(err)
}

func (this *Client) CreateFolderIfNotExist(path string) error {
	ok, err := this.Exist(path)
	if !ok && err == nil {
		err = this.CreateFolder(path)
	}
	return err
}

func (this *Client) CreateFolderIfNotExistMust(path string) {
	err := this.CreateFolderIfNotExist(path)
	moon.Check(err)
}

func (this *Client) UploadFile(localPath, remotePath string) error {
	// функция получения url для загрузки файла
	getUploadUrl := func(path string) (string, error) {
		res, err := this.apiRequest(fmt.Sprintf("resources/upload?path=%s&overwrite=true", path), "GET")
		if err != nil {
			return "", err
		}
		var resultJson struct {
			Href string `json:"href"`
		}
		err = json.NewDecoder(res.Body).Decode(&resultJson)
		if err != nil {
			return "", err
		}
		return resultJson.Href, err
	}

	// читаем локальный файл с диска
	data, err := os.Open(localPath)
	if err != nil {
		return err
	}
	// получем ссылку для загрузки файла
	href, err := getUploadUrl(remotePath)
	if err != nil {
		return err
	}
	defer data.Close()
	// загружаем файл по полученной ссылке методом PUT
	req, err := http.NewRequest("PUT", href, data)
	if err != nil {
		return err
	}
	// в header запроса добавляем токен
	req.Header.Add("Authorization", fmt.Sprintf("OAuth %s", this.authToken))

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

func (this *Client) UploadFileMust(localPath, remotePath string) {
	err := this.UploadFile(localPath, remotePath)
	moon.Check(err)
}

func (this *Client) DeleteFile(path string) error {
	_, err := this.apiRequest(fmt.Sprintf("resources?path=%s&permanently=true", path), "DELETE")
	return err
}

func (this *Client) DeleteFileMust(path string) {
	err := this.DeleteFile(path)
	moon.Check(err)
}

func (this *Client) GetResource(path string) (*Resource, error) {
	res, err := this.apiRequest(fmt.Sprintf("resources?path=%s&limit=50&sort=-created", path), "GET")
	if err != nil {
		return nil, err
	}

	var result *Resource
	err = json.NewDecoder(res.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (this *Client) GetResourceMust(path string) *Resource {
	res, err := this.GetResource(path)
	moon.Check(err)
	return res
}

func (this *Client) Exist(path string) (ok bool, err error) {
	_, err = this.GetResource(path)
	if err == nil {
		ok = true
	} else {
		if e, ok := err.(*APIError); ok && e.Code == "DiskNotFoundError" {
			err = nil
		}
	}
	return
}

func (this *Client) ExistMust(path string) bool {
	ok, err := this.Exist(path)
	moon.Check(err)
	return ok
}

func checkResponse(r *http.Response) error {
	if r.StatusCode >= 400 {
		apiErr := new(APIError)
		// Skipping the json decoding errors.
		json.NewDecoder(r.Body).Decode(apiErr)
		return apiErr
	}
	return nil
}
