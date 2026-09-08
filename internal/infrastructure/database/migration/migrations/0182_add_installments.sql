-- 0182: Add Installment table
--
-- Supports splitting a large Payment (e.g. a yearly subscription or course
-- bundle) into scheduled installments. Each row is one due/paid installment
-- linked back to the originating Payment and the paying User.

CREATE TABLE public."Installment" (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    payment_id uuid,
    installment_number integer NOT NULL,
    total_installments integer NOT NULL,
    amount numeric(19,4) NOT NULL,
    currency text DEFAULT 'EGP'::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    due_date timestamp with time zone NOT NULL,
    paid_at timestamp with time zone,
    method text,
    reference text,
    notes text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    deleted_at timestamp with time zone,
    CONSTRAINT "Installment_pkey" PRIMARY KEY (id),
    CONSTRAINT "Installment_user_id_fkey" FOREIGN KEY (user_id) REFERENCES public."User"(id) ON DELETE CASCADE,
    CONSTRAINT "Installment_payment_id_fkey" FOREIGN KEY (payment_id) REFERENCES public."Payment"(id) ON DELETE SET NULL,
    CONSTRAINT chk_installment_amount_nonnegative CHECK ((amount >= (0)::numeric)),
    CONSTRAINT chk_installment_status_valid CHECK ((status = ANY (ARRAY['pending'::text, 'paid'::text, 'overdue'::text, 'cancelled'::text])))
);

CREATE INDEX idx_installment_user_id ON public."Installment" (user_id);
CREATE INDEX idx_installment_payment_id ON public."Installment" (payment_id);
CREATE INDEX idx_installment_status ON public."Installment" (status);
CREATE INDEX idx_installment_due_date ON public."Installment" (due_date);

ALTER TABLE public."Installment" DISABLE ROW LEVEL SECURITY;
