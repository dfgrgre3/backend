-- 0184: Add content management tables
--
-- Banner: promotional banners shown across the site.
-- FAQ: frequently asked questions grouped by category.
-- HomepageSection: ordered, toggleable sections of the public homepage.
-- CMSPage: freeform static pages (About, Terms, Privacy, ...), replacing
-- the previous stub admin/cms/pages handlers with real persistence.

CREATE TABLE public."Banner" (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    title text NOT NULL,
    image_url text NOT NULL,
    link_url text,
    "position" text DEFAULT 'home_top'::text NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    display_order integer DEFAULT 0 NOT NULL,
    start_date timestamp with time zone,
    end_date timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    deleted_at timestamp with time zone,
    CONSTRAINT "Banner_pkey" PRIMARY KEY (id)
);
CREATE INDEX idx_banner_position ON public."Banner" ("position");
CREATE INDEX idx_banner_is_active ON public."Banner" (is_active);
ALTER TABLE public."Banner" DISABLE ROW LEVEL SECURITY;

CREATE TABLE public."FAQ" (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    question text NOT NULL,
    answer text NOT NULL,
    category text DEFAULT 'general'::text NOT NULL,
    display_order integer DEFAULT 0 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    deleted_at timestamp with time zone,
    CONSTRAINT "FAQ_pkey" PRIMARY KEY (id)
);
CREATE INDEX idx_faq_category ON public."FAQ" (category);
ALTER TABLE public."FAQ" DISABLE ROW LEVEL SECURITY;

CREATE TABLE public."HomepageSection" (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    key text NOT NULL,
    type text NOT NULL,
    title text,
    content text,
    is_active boolean DEFAULT true NOT NULL,
    display_order integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    deleted_at timestamp with time zone,
    CONSTRAINT "HomepageSection_pkey" PRIMARY KEY (id),
    CONSTRAINT "HomepageSection_key_key" UNIQUE (key)
);
ALTER TABLE public."HomepageSection" DISABLE ROW LEVEL SECURITY;

CREATE TABLE public."CMSPage" (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    slug text NOT NULL,
    title text NOT NULL,
    content text,
    status text DEFAULT 'draft'::text NOT NULL,
    meta_title text,
    meta_description text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    deleted_at timestamp with time zone,
    CONSTRAINT "CMSPage_pkey" PRIMARY KEY (id),
    CONSTRAINT "CMSPage_slug_key" UNIQUE (slug),
    CONSTRAINT chk_cms_page_status_valid CHECK ((status = ANY (ARRAY['draft'::text, 'published'::text, 'archived'::text])))
);
ALTER TABLE public."CMSPage" DISABLE ROW LEVEL SECURITY;
