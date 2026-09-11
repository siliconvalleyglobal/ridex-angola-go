package angola

import (
	"fmt"
	"sync"
	"time"
)

// Translator provides Portuguese (Angola) translations.
type Translator struct {
	mu    sync.RWMutex
	cache map[string]string
}

// NewTranslator creates a new Portuguese translator for Angola.
func NewTranslator() *Translator {
	return &Translator{
		cache: loadTranslations(),
	}
}

// T returns the translated message for the given key.
func (t *Translator) T(key string, args ...interface{}) string {
	t.mu.RLock()
	msg, ok := t.cache[key]
	t.mu.RUnlock()

	if !ok {
		if len(args) > 0 {
			return fmt.Sprintf("[%s] %s", key, fmt.Sprint(args...))
		}
		return "[" + key + "]"
	}

	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}

	return msg
}

func loadTranslations() map[string]string {
	return map[string]string{
		MsgRideRequested:     "Pedido de corrida recebido com sucesso.",
		MsgDriverFound:       "Motorista encontrado. Agora vai para o seu encontro.",
		MsgDriverAccepting:   "Um motorista está a aceitar o pedido de corrida.",
		MsgDriverOnWay:       "O motorista está a caminho do seu local de recolha.",
		MsgDriverArriving:    "O motorista está a chegar ao seu local de recolha.",
		MsgRideStarted:       "A corrida começou. Boa viagem!",
		MsgRideCompleted:     "A corrida foi concluída. Obrigado pela utilização do RideX!",
		MsgRideCancelled:     "A corrida foi cancelada.",
		MsgRideNoDriverFound: "Não encontramos motoristas disponíveis nesta área. Por favor, tente novamente mais tarde.",
		MsgRideExpired:       "O pedido de corrida expirou. Por favor, tente novamente.",
		MsgRidePinRequired:   "É necessário introduzir o PIN do passeio para continuar.",
		MsgRidePinVerified:   "PIN verificado com sucesso.",
		MsgRidePinInvalid:    "PIN inválido. Por favor, tente novamente.",

		MsgPaymentPending:    "O pagamento está pendente.",
		MsgPaymentProcessing: "A processar pagamento...",
		MsgPaymentCompleted:  "Pagamento concluído com sucesso.",
		MsgPaymentFailed:     "O pagamento falhou. Por favor, tente novamente.",
		MsgPaymentRefunded:   "Reembolso processado com sucesso.",
		MsgPaymentCash:       "Pagamento em dinheiro.",
		MsgPaymentCard:       "Pagamento com cartão.",
		MsgPaymentMulticaixa: "Pagamento com Multicaixa.",
		MsgPaymentChangeDue:  "Troco a receber: %s AOA",
		MsgPaymentAmountDue:  "Valor a pagar: %s AOA",

		MsgInvoiceCreated:   "Fatura criada com sucesso.",
		MsgInvoiceSent:      "Fatura enviada com sucesso.",
		MsgInvoiceAvailable: "A sua fatura está disponível para download.",

		MsgKycSubmitted: "Documentos KYC enviados para revisão.",
		MsgKycApproved:  "Os seus documentos KYC foram aprovados. Agora pode usar a aplicação.",
		MsgKycRejected:  "Os seus documentos KYC foram rejeitados. Por favor, verifique os requisitos e tente novamente.",
		MsgKycPending:   "A revisar os seus documentos KYC...",
		MsgKycRequired:  "É necessário completar a verificação KYC para continuar.",

		MsgOTPWelcome:          "Bem-vindo ao RideX Angola! Entrou em contato com o código de verificação.",
		MsgOTPReceived:         "Código de verificação recebido.",
		MsgOTPSent:             "Código de verificação enviado para o seu telemóvel.",
		MsgOTPExpired:          "O código de verificação expirou. Por favor, solicite um novo código.",
		MsgOTPInvalid:          "Código de verificação inválido. Por favor, tente novamente.",
		MsgOTPRequired:         "É necessário um código de verificação para continuar.",
		MsgOTPTooManyAttempts:  "Demasiados tentativas falhadas. Por favor, aguarde %d minutos antes de tentar novamente.",
		MsgOTPRateLimited:      "Demasiados pedidos de código. Por favor, aguarde %d minutos antes de solicitar um novo código.",
		MsgOTPPhoneChanged:     "O seu número de telemóvel foi alterado com sucesso.",
		MsgOTPVerificationCode: "O seu código de verificação é: %s",

		MsgOTPLogin:            "Código OTP para login RideX Angola: %s. Valido por 5 minutos.",
		MsgOTPRegister:         "Código OTP para registo RideX Angola: %s. Valido por 5 minutos.",
		MsgOTPPasswordReset:    "Código OTP para redefinição de password RideX Angola: %s. Valido por 5 minutos.",
		MsgOTPPhoneChange:      "Código OTP para alterar telemóvel RideX Angola: %s. Valido por 5 minutos.",
		MsgAuthWelcome:         "Bem-vindo ao RideX Angola!",
		MsgAuthLoginSuccess:    "Login realizado com sucesso.",
		MsgAuthLogoutSuccess:   "Falha de login realizada com sucesso.",
		MsgAuthTokenExpired:    "A sessão expirou. Por favor, inicie sessão novamente.",
		MsgAuthUnauthorized:    "Não autorizado. Por favor, inicie sessão para continuar.",
		MsgAuthForbidden:       "Acesso negado. Não tem permissão para executar esta ação.",
		MsgAuthPasswordReset:   "Password resetada com sucesso. Por favor, inicie sessão com a nova password.",
		MsgAuthPasswordChanged: "Password alterada com sucesso.",
		MsgAuthAccountLocked:   "A sua conta foi bloqueada temporariamente devido a múltiplas falhas de login.",

		MsgOfferReceived:       "Oferta de corrida recebida.",
		MsgOfferAccepted:       "Oferta aceite com sucesso.",
		MsgOfferDeclined:       "Oferta recusada com sucesso.",
		MsgOfferExpired:        "A oferta de corrida expirou.",
		MsgOfferNoDrivers:      "Sem motoristas disponíveis para aceitar ofertas no momento.",
		MsgOfferTooManyDrivers: "Demasiados motoristas disponíveis. Por favor, tente novamente mais tarde.",

		MsgSafetySOSTriggered:    "SOS acionado. Estamos a enviar ajuda imediatamente.",
		MsgSafetyContactNotified: "Os seus contactos de emergência foram notificados.",
		MsgSafetyTripShared:      "Viagem partilhada com os seus contactos de emergência.",
		MsgSafetyEmergency:       "Emergência. Ligue para os serviços de emergência ou utilize o botão SOS.",

		MsgSupportTicketCreated:    "Ticket de suporte criado com sucesso.",
		MsgSupportTicketUpdated:    "Ticket de suporte atualizado com sucesso.",
		MsgSupportResponseReceived: "Resposta do suporte recebida.",

		MsgGenericError:        "Ocorreu um erro. Por favor, tente novamente mais tarde.",
		MsgGenericSuccess:      "Operação realizada com sucesso.",
		MsgGenericNotFound:     "Não encontrado.",
		MsgGenericUnauthorized: "Não autorizado.",
		MsgGenericValidation:   "Erro de validação.",
		MsgGenericNetworkError: "Erro de rede. Por favor, verifique a sua ligação e tente novamente.",
		MsgGenericTimeout:      "Tempo limite excedido. Por favor, tente novamente.",
	}
}

// FormatCurrency formats an amount in Angolan Kwanza (AOA).
func FormatCurrency(amount int64) string {
	return fmt.Sprintf("%.2f AOA", float64(amount)/100.0)
}

// FormatPhone formats Angolan phone numbers in international format.
func FormatPhone(phone string) string {
	var digits string
	for _, c := range phone {
		if c >= '0' && c <= '9' {
			digits += string(c)
		}
	}

	if len(digits) > 0 && digits[0] == '0' {
		digits = "244" + digits[1:]
	}

	if len(digits) <= 9 || (len(digits) > 3 && digits[:3] != "244") {
		digits = "244" + digits
	}

	if len(digits) == 12 && digits[:3] == "244" {
		return fmt.Sprintf("+%s %s %s %s",
			digits[:3],
			digits[3:6],
			digits[6:9],
			digits[9:12])
	}

	return "+" + digits
}

// NormalizePhone normalizes an Angolan phone number to E.164 format.
func NormalizePhone(phone string) (string, error) {
	var digits string
	for _, c := range phone {
		if c == '+' || (c >= '0' && c <= '9') {
			digits += string(c)
		}
	}

	if len(digits) > 0 && digits[0] == '+' {
		digits = digits[1:]
	}

	if len(digits) >= 2 && digits[:2] == "00" {
		digits = "244" + digits[2:]
	}

	if len(digits) > 0 && digits[0] == '0' {
		if len(digits) >= 3 {
			prefix := digits[1:3]
			if prefix == "91" || prefix == "92" || prefix == "93" || prefix == "94" || prefix == "95" {
				digits = "244" + digits[1:]
			} else if len(digits) >= 4 && digits[1:4] == "222" {
				digits = "244" + digits[1:]
			}
		}
	}

	if len(digits) <= 9 || (len(digits) > 3 && digits[:3] != "244") {
		digits = "244" + digits
	}

	if len(digits) != 12 {
		return "", fmt.Errorf("número de telemóvel inválido: %s", phone)
	}

	return digits, nil
}

// ParseISODate parses a date string in ISO format and returns a formatted string.
func ParseISODate(dateStr string) string {
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return dateStr
	}

	return t.Format("02/01/2006 15:04")
}
