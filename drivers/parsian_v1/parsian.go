package parsian_v1

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"gopay"
	"io"
	"net/http"
)

// =======================
// 📦 ساختار درایور پارسیان
// =======================

type Driver struct {
	LoginAccount string
}

// =======================
// 🏗️ تابع سازنده درایور
// =======================

func New(config gopay.DriverConfig) (gopay.RedirectPayer, error) {
	login := config["login_account"]
	if login == "" {
		return nil, errors.New("missing login_account in config")
	}
	return &Driver{LoginAccount: login}, nil
}

// =======================
// 💳 مرحله ۱: ایجاد تراکنش و دریافت توکن پرداخت
// =======================

func (d *Driver) Purchase(ctx context.Context, req *gopay.TransactionRequest) (*gopay.PaymentResponse, error) {
	soapBody := fmt.Sprintf(`
	<soap:Envelope xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
		xmlns:xsd="http://www.w3.org/2001/XMLSchema"
		xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
		<soap:Body>
			<SalePaymentRequest xmlns="https://pec.Shaparak.ir/NewIPGServices/Sale/SaleService">
				<requestData>
					<LoginAccount>%s</LoginAccount>
					<Amount>%d</Amount>
					<OrderId>%s</OrderId>
					<CallBackUrl>%s</CallBackUrl>
					<AdditionalData></AdditionalData>
				</requestData>
			</SalePaymentRequest>
		</soap:Body>
	</soap:Envelope>`, d.LoginAccount, req.Amount, req.IdempotencyKey, req.CallbackURL)

	httpReq, _ := http.NewRequestWithContext(ctx, "POST",
		"https://pec.shaparak.ir/NewIPGServices/Sale/SaleService.asmx",
		bytes.NewBuffer([]byte(soapBody)))

	httpReq.Header.Set("Content-Type", "text/xml; charset=utf-8")
	httpReq.Header.Set("SOAPAction",
		"https://pec.Shaparak.ir/NewIPGServices/Sale/SaleService/SalePaymentRequest")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Parse XML Response
	type SalePaymentResult struct {
		Token   int64  `xml:"Token"`
		Message string `xml:"Message"`
		Status  int    `xml:"Status"`
	}
	type SalePaymentResponse struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Response struct {
				Result SalePaymentResult `xml:"SalePaymentRequestResult"`
			} `xml:"SalePaymentRequestResponse"`
		} `xml:"Body"`
	}

	var parsed SalePaymentResponse
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("xml parse error: %v", err)
	}

	result := parsed.Body.Response.Result
	if result.Status != 0 {
		return nil, fmt.Errorf("parsian error: %s", parsianStatusToMessage(result.Status))
	}

	token := result.Token
	paymentURL := fmt.Sprintf("https://pec.shaparak.ir/NewIPG/?Token=%d", token)

	return &gopay.PaymentResponse{
		Success:    true,
		Message:    result.Message,
		Authority:  fmt.Sprintf("%d", token),
		PaymentURL: paymentURL,
	}, nil
}

// =======================
// 🔍 مرحله ۲: تأیید و نهایی‌سازی پرداخت (ConfirmPayment)
// =======================

func (d *Driver) VerifyAndConfirm(ctx context.Context, r *http.Request, fetcher gopay.TransactionFetcher) (*gopay.VerificationResponse, error) {
	token := r.FormValue("Token")
	if token == "" {
		return nil, errors.New("missing Token in callback request")
	}

	confirmBody := fmt.Sprintf(`
	<soap:Envelope xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
	xmlns:xsd="http://www.w3.org/2001/XMLSchema"
	xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
	  <soap:Body>
		<ConfirmPayment xmlns="https://pec.Shaparak.ir/NewIPGServices/Confirm/ConfirmService">
		  <requestData>
			<LoginAccount>%s</LoginAccount>
			<Token>%s</Token>
		  </requestData>
		</ConfirmPayment>
	  </soap:Body>
	</soap:Envelope>`, d.LoginAccount, token)

	reqConfirm, _ := http.NewRequestWithContext(ctx, "POST",
		"https://pec.shaparak.ir/NewIPGServices/Confirm/ConfirmService.asmx",
		bytes.NewBuffer([]byte(confirmBody)))

	reqConfirm.Header.Set("Content-Type", "text/xml; charset=utf-8")
	reqConfirm.Header.Set("SOAPAction",
		"https://pec.Shaparak.ir/NewIPGServices/Confirm/ConfirmService/ConfirmPayment")

	res, err := http.DefaultClient.Do(reqConfirm)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	// Parse ConfirmPaymentResponse
	type ConfirmResult struct {
		Status           int    `xml:"Status"`
		CardNumberMasked string `xml:"CardNumberMasked"`
		RRN              int64  `xml:"RRN"`
		Message          string `xml:"Message"`
	}
	type ConfirmEnvelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Response struct {
				Result ConfirmResult `xml:"ConfirmPaymentResult"`
			} `xml:"ConfirmPaymentResponse"`
		} `xml:"Body"`
	}

	var confirm ConfirmEnvelope
	if err := xml.Unmarshal(body, &confirm); err != nil {
		return nil, fmt.Errorf("xml parse error: %v", err)
	}

	result := confirm.Body.Response.Result
	if result.Status != 0 {
		return &gopay.VerificationResponse{
			Status:  gopay.StatusFailed,
			Message: parsianStatusToMessage(result.Status),
		}, nil
	}

	return &gopay.VerificationResponse{
		Status:      gopay.StatusSuccess,
		ReferenceID: fmt.Sprintf("%d", result.RRN),
		CardNumber:  result.CardNumberMasked,
	}, nil
}

// =======================
// 📛 نام درایور برای لاگ یا فکتوری
// =======================

func (d *Driver) GetName() string {
	return "parsian_v1"
}

// =======================
// ⚙️ نگاشت کدهای خطای پارسیان به پیام‌های فارسی
// =======================

func parsianStatusToMessage(status int) string {
	switch status {
	case 0:
		return "عملیات با موفقیت انجام شد"
	case -1:
		return "خطای داخلی سرور بانک پارسیان"
	case -2:
		return "تراکنش تکراری یا نامعتبر"
	case -3:
		return "پاسخ نامعتبر از سامانه مرکزی"
	case -100:
		return "پذیرنده غیرفعال است"
	case -101:
		return "پذیرنده احراز هویت نشد (LoginAccount یا IP نادرست است)"
	case -102:
		return "اطلاعات درخواست ناقص یا نادرست است"
	case -111:
		return "مبلغ تراکنش بیش از سقف مجاز پذیرنده است"
	case -112:
		return "شماره سفارش تکراری است"
	case -127:
		return "آدرس IP شما در لیست سفید بانک نیست"
	case -138:
		return "پرداخت توسط کاربر لغو شد"
	case -1551:
		return "برگشت تراکنش قبلاً انجام شده است"
	default:
		if status > 0 {
			return fmt.Sprintf("کد خطای شاپرک: %d — لطفاً وضعیت تراکنش را از شاپرک بررسی کنید", status)
		}
		return fmt.Sprintf("خطای ناشناخته با کد: %d", status)
	}
}
