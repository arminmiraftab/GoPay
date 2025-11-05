package parsian_v1

//
//import (
//	"context"
//	"encoding/xml"
//	"errors"
//	"fmt"
//	"gopay"
//	"net/http"
//)
//
//type Driver struct {
//	client *SOAPClient
//}
//
//func New(config gopay.DriverConfig) (gopay.RedirectPayer, error) {
//	login := config["login_account"]
//	if login == "" {
//		return nil, errors.New("login_account is required")
//	}
//	return &Driver{
//		client: &SOAPClient{LoginAccount: login},
//	}, nil
//}
//
//// Purchase => مرحله اول درخواست توکن پرداخت
//func (d *Driver) Purchase(ctx context.Context, req *gopay.TransactionRequest) (*gopay.PaymentResponse, error) {
//	resp, err := d.client.SalePaymentRequest(req.Amount, req.IdempotencyKey, req.CallbackURL)
//	if err != nil {
//		return nil, err
//	}
//
//	if resp.Status != 0 {
//		return nil, fmt.Errorf("parsian sale failed: %s", resp.Message)
//	}
//
//	paymentURL := fmt.Sprintf("https://pec.shaparak.ir/NewIPG/?Token=%d", resp.Token)
//	return &gopay.PaymentResponse{
//		Success:    true,
//		Message:    "عملیات موفق",
//		Authority:  fmt.Sprintf("%d", resp.Token),
//		PaymentURL: paymentURL,
//	}, nil
//}
//
//// VerifyAndConfirm => تأیید پرداخت پس از بازگشت از بانک
//func (d *Driver) VerifyAndConfirm(ctx context.Context, r *http.Request, fetcher gopay.TransactionFetcher) (*gopay.VerificationResponse, error) {
//	token := r.FormValue("Token")
//	if token == "" {
//		return nil, errors.New("missing token in callback")
//	}
//
//	body, err := d.client.ConfirmPayment(token)
//	if err != nil {
//		return nil, err
//	}
//
//	var env confirmEnvelope
//	if err := xml.Unmarshal(body, &env); err != nil {
//		return nil, fmt.Errorf("parse error: %v", err)
//	}
//
//	result := env.Body.Response.Result
//	if result.Status != 0 {
//		return &gopay.VerificationResponse{Status: gopay.StatusFailed}, nil
//	}
//
//	return &gopay.VerificationResponse{
//		Status:      gopay.StatusSuccess,
//		ReferenceID: fmt.Sprintf("%d", result.RRN),
//		CardNumber:  result.CardNumberMasked,
//	}, nil
//}
//
//type confirmEnvelope struct {
//	XMLName xml.Name `xml:"Envelope"`
//	Body    struct {
//		Response struct {
//			Result struct {
//				Status           int    `xml:"Status"`
//				RRN              int64  `xml:"RRN"`
//				CardNumberMasked string `xml:"CardNumberMasked"`
//			} `xml:"ConfirmPaymentResult"`
//		} `xml:"ConfirmPaymentResponse"`
//	} `xml:"Body"`
//}
