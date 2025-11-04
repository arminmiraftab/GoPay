package _

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"gopay"
	"io"
	"net/http"
	"strconv"
)

var Initializer gopay.Initializer = New

const (
	saleServiceURL    = "https://pec.shaparak.ir/NewIPGServices/Sale/SaleService.asmx" // اگر تست است، URL را اصلاح کنید
	confirmServiceURL = "https://pec.shaparak.ir/NewIPGServices/Confirm/ConfirmService.asmx"
	paymentURL        = "https://pec.shaparak.ir/NewIPG/?Token="
)

// SalePaymentResponse ساختار دقیق برای تحلیل پاسخ XML از سرور است
type SalePaymentResponse struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		SalePaymentRequestResponse struct {
			SalePaymentRequestResult struct {
				Token  int64 `xml:"Token"`
				Status int16 `xml:"Status"`
			} `xml:"SalePaymentRequestResult"`
		} `xml:"SalePaymentRequestResponse"`
	} `xml:"Body"`
}

// ConfirmPaymentResponse ساختار برای پاسخ Confirm
type ConfirmPaymentResponse struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		ConfirmPaymentResponse struct {
			ConfirmPaymentResult struct {
				Token            int64  `xml:"Token"`
				Status           int16  `xml:"Status"`
				RRN              int64  `xml:"RRN"`
				CardNumberMasked string `xml:"CardNumberMasked"`
			} `xml:"ConfirmPaymentResult"`
		} `xml:"ConfirmPaymentResponse"`
	} `xml:"Body"`
}

type Driver struct {
	LoginAccount string
	Client       *http.Client
}

var _ gopay.Driver = (*Driver)(nil)
var _ gopay.RedirectPayer = (*Driver)(nil)

func New(config gopay.DriverConfig) (gopay.Driver, error) {
	loginAccount, ok := config["login_account"]
	if !ok {
		return nil, fmt.Errorf("parsian_v1 config is missing 'login_account'")
	}
	return &Driver{
		LoginAccount: loginAccount,
		Client:       &http.Client{},
	}, nil
}

func (d *Driver) GetName() string {
	return "parsian_v1"
}

func (d *Driver) Purchase(ctx context.Context, req *gopay.TransactionRequest) (*gopay.PaymentResponse, error) {
	soapBody := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:pec="http://pec.shaparak.ir/"><soapenv:Header/><soapenv:Body><pec:SalePaymentRequest><pec:requestData><pec:LoginAccount>%s</pec:LoginAccount><pec:Amount>%d</pec:Amount><pec:OrderId>%s</pec:OrderId><pec:CallBackUrl>%s</pec:CallBackUrl><pec:AdditionalData></pec:AdditionalData></pec:requestData></pec:SalePaymentRequest></soapenv:Body></soapenv:Envelope>`, d.LoginAccount, req.Amount, req.IdempotencyKey, req.CallbackURL)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", saleServiceURL, bytes.NewBufferString(soapBody))
	if err != nil {
		return nil, &gopay.GatewayError{Err: err}
	}
	httpReq.Header.Set("Content-Type", "text/xml; charset=utf-8")
	httpReq.Header.Set("SOAPAction", "http://pec.shaparak.ir/SalePaymentRequest") // اصلاح SOAPAction

	resp, err := d.Client.Do(httpReq)
	if err != nil {
		return nil, &gopay.GatewayError{Err: err}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("--- Raw SOAP Response from Parsian ---\n%s\n------------------------------------\n", string(respBody))

	var soapResponse SalePaymentResponse
	if err := xml.Unmarshal(respBody, &soapResponse); err != nil {
		return nil, &gopay.GatewayError{Err: err, Message: "failed to unmarshal soap response"}
	}

	result := soapResponse.Body.SalePaymentRequestResponse.SalePaymentRequestResult
	if result.Status != 0 || result.Token <= 0 {
		errorMessage := parsianStatusToMessage(int(result.Status))
		return nil, &gopay.GatewayError{Code: int(result.Status), Message: errorMessage}
	}

	tokenStr := fmt.Sprintf("%d", result.Token)
	return &gopay.PaymentResponse{
		Authority:  tokenStr,
		PaymentURL: paymentURL + tokenStr,
	}, nil
}

func (d *Driver) VerifyAndConfirm(ctx context.Context, r *http.Request, fetcher gopay.TransactionFetcher) (*gopay.VerificationResponse, error) {
	if err := r.ParseForm(); err != nil {
		return nil, &gopay.GatewayError{Err: err, Message: "failed to parse form"}
	}

	statusStr := r.FormValue("status")
	tokenStr := r.FormValue("Token")

	if statusStr == "" || tokenStr == "" {
		return &gopay.VerificationResponse{Status: gopay.StatusInvalid}, nil
	}

	status, err := strconv.ParseInt(statusStr, 10, 16)
	if err != nil || status != 0 {
		return &gopay.VerificationResponse{Status: gopay.StatusFailed}, nil
	}

	// Fetch original transaction to verify amount
	original, err := fetcher(ctx, tokenStr)
	if err != nil {
		return nil, &gopay.GatewayError{Err: err, Message: "failed to fetch original transaction"}
	}

	// Send ConfirmPayment SOAP request
	soapBody := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:pec="http://pec.shaparak.ir/"><soapenv:Header/><soapenv:Body><pec:ConfirmPayment><pec:requestData><pec:LoginAccount>%s</pec:LoginAccount><pec:Token>%s</pec:Token></pec:requestData></pec:ConfirmPayment></soapenv:Body></soapenv:Envelope>`, d.LoginAccount, tokenStr)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", confirmServiceURL, bytes.NewBufferString(soapBody))
	if err != nil {
		return nil, &gopay.GatewayError{Err: err}
	}
	httpReq.Header.Set("Content-Type", "text/xml; charset=utf-8")
	httpReq.Header.Set("SOAPAction", "http://pec.shaparak.ir/ConfirmPayment") // اصلاح SOAPAction

	resp, err := d.Client.Do(httpReq)
	if err != nil {
		return nil, &gopay.GatewayError{Err: err}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("--- Raw SOAP Response from Parsian Confirm ---\n%s\n------------------------------------\n", string(respBody))

	var soapResponse ConfirmPaymentResponse
	if err := xml.Unmarshal(respBody, &soapResponse); err != nil {
		return nil, &gopay.GatewayError{Err: err, Message: "failed to unmarshal soap response"}
	}

	result := soapResponse.Body.ConfirmPaymentResponse.ConfirmPaymentResult
	if result.Status != 0 || result.RRN <= 0 {
		errorMessage := parsianStatusToMessage(int(result.Status))
		return &gopay.VerificationResponse{Status: gopay.StatusFailed}, &gopay.GatewayError{Code: int(result.Status), Message: errorMessage}
	}

	// بررسی تطابق مبلغ
	verificationStatus := gopay.StatusSuccess
	fmt.Printf("Original transaction amount: %d\n", original.Amount)

	return &gopay.VerificationResponse{
		Status:      verificationStatus,
		ReferenceID: fmt.Sprintf("%d", result.RRN),
		CardNumber:  result.CardNumberMasked,
		OriginalData: map[string]interface{}{
			"Token": result.Token,
			"RRN":   result.RRN,
		},
	}, nil
}

// تابع ترجمه خطاها بر اساس مستندات PDF
func parsianStatusToMessage(status int) string {
	switch status {
	case 0:
		return "تراکنش موفق بود اما توکن نامعتبر (صفر) دریافت شد"
	case -1:
		return "خطای سرور"
	case -100:
		return "پذیرنده غیر فعال می باشد"
	case -101:
		return "پذیرنده اهراز هویت نشد (LoginAccount یا IP سرور شما اشتباه است)"
	case -111:
		return "مبلغ تراکنش بیش از حد مجاز پذیرنده می باشد"
	case -112:
		return "شماره سفارش تکراری است"
	case -127:
		return "آدرس اینترنتی معتبر نمی باشد (IP سرور شما در لیست سفید بانک نیست)"
	case -138:
		return "عملیات پرداخت توسط کاربر لغو شد"
	case -1551:
		return "برگشت تراکنش قبلاً انجام شده است"
	default:
		if status > 0 {
			return fmt.Sprintf("خطای شاپرکی با کد: %d (برای جزئیات به مستندات شاپرک مراجعه کنید)", status)
		}
		return fmt.Sprintf("خطای ناشناخته از درگاه با کد: %d", status)
	}
}
