package aliyun

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/alidns"
	"github.com/qdm12/ddns-updater/internal/models"
	"github.com/qdm12/ddns-updater/internal/settings/constants"
	"github.com/qdm12/ddns-updater/internal/settings/errors"
	"github.com/qdm12/ddns-updater/internal/settings/utils"
	"github.com/qdm12/ddns-updater/pkg/publicip/ipversion"
)

type Provider struct {
	domain       string
	host         string
	ipVersion    ipversion.IPVersion
	accessKeyID  string
	accessSecret string
	region       string
}

func New(data json.RawMessage, domain, host string,
	ipVersion ipversion.IPVersion) (p *Provider, err error) {
	extraSettings := struct {
		AccessKeyID  string `json:"access_key_id"`
		AccessSecret string `json:"access_secret"`
		Region       string `json:"region"`
	}{}
	if err := json.Unmarshal(data, &extraSettings); err != nil {
		return nil, err
	}
	p = &Provider{
		domain:       domain,
		host:         host,
		ipVersion:    ipVersion,
		accessKeyID:  extraSettings.AccessKeyID,
		accessSecret: extraSettings.AccessSecret,
		region:       "cn-hangzhou",
	}
	if extraSettings.Region != "" {
		p.region = extraSettings.Region
	}
	if err := p.isValid(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Provider) isValid() error {
	switch {
	case p.accessKeyID == "":
		return errors.ErrEmptyAccessKeyID
	case p.accessSecret == "":
		return errors.ErrEmptyAccessKeySecret
	}
	return nil
}

func (p *Provider) String() string {
	return utils.ToString(p.domain, p.host, constants.Aliyun, p.ipVersion)
}

func (p *Provider) Domain() string {
	return p.domain
}

func (p *Provider) Host() string {
	return p.host
}

func (p *Provider) IPVersion() ipversion.IPVersion {
	return p.ipVersion
}

func (p *Provider) Proxied() bool {
	return false
}

func (p *Provider) BuildDomainName() string {
	return utils.BuildDomainName(p.host, p.domain)
}

func (p *Provider) HTML() models.HTMLRow {
	return models.HTMLRow{
		Domain:    models.HTML(fmt.Sprintf("<a href=\"http://%s\">%s</a>", p.BuildDomainName(), p.BuildDomainName())),
		Host:      models.HTML(p.Host()),
		Provider:  "<a href=\"https://www.aliyun.com/\">Aliyun</a>",
		IPVersion: models.HTML(p.ipVersion.String()),
	}
}

func (p *Provider) Update(ctx context.Context, _ *http.Client, ip net.IP) (newIP net.IP, err error) {
	recordType := constants.A
	if ip.To4() == nil {
		recordType = constants.AAAA
	}

	client, err := alidns.NewClientWithAccessKey(p.region, p.accessKeyID, p.accessSecret)
	if err != nil {
		return nil, err
	}

	listRequest := alidns.CreateDescribeDomainRecordsRequest()
	listRequest.Scheme = "https"

	listRequest.DomainName = p.domain
	listRequest.RRKeyWord = p.host
	resp, err := client.DescribeDomainRecords(listRequest)
	if err != nil {
		return nil, err
	}
	recordID := ""
	for _, record := range resp.DomainRecords.Record {
		if strings.EqualFold(record.RR, p.host) {
			recordID = record.RecordId
			break
		}
	}
	if recordID == "" {
		return nil, errors.ErrRecordNotFound
	}

	request := alidns.CreateUpdateDomainRecordRequest()
	request.Scheme = "https"

	request.Value = ip.String()
	request.Type = recordType
	request.RR = p.host
	request.RecordId = recordID

	_, err = client.UpdateDomainRecord(request)
	return ip, err
}
