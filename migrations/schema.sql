--
-- PostgreSQL database dump
--

-- Dumped from database version 16.8 (Debian 16.8-1.pgdg120+1)
-- Dumped by pg_dump version 16.8 (Ubuntu 16.8-1.pgdg24.04+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: public; Type: SCHEMA; Schema: -; Owner: jofosuware
--

-- *not* creating schema, since initdb creates it


ALTER SCHEMA public OWNER TO jofosuware;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: avatar; Type: TABLE; Schema: public; Owner: jofosuware
--

CREATE TABLE public.avatar (
    public_id character varying(255) NOT NULL,
    url character varying(255) NOT NULL,
    user_id uuid NOT NULL
);


ALTER TABLE public.avatar OWNER TO jofosuware;

--
-- Name: images; Type: TABLE; Schema: public; Owner: jofosuware
--

CREATE TABLE public.images (
    public_id character varying(300) NOT NULL,
    url character varying(300) NOT NULL,
    product_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.images OWNER TO jofosuware;

--
-- Name: order_items; Type: TABLE; Schema: public; Owner: jofosuware
--

CREATE TABLE public.order_items (
    item_id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name character varying(100) NOT NULL,
    quantity integer NOT NULL,
    image character varying(1000) NOT NULL,
    price integer NOT NULL,
    product_id uuid NOT NULL,
    order_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT order_items_name_check CHECK (((name)::text <> ''::text))
);


ALTER TABLE public.order_items OWNER TO jofosuware;

--
-- Name: orders; Type: TABLE; Schema: public; Owner: jofosuware
--

CREATE TABLE public.orders (
    order_id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    item_price integer NOT NULL,
    tax_price integer NOT NULL,
    shipping_price integer NOT NULL,
    total_price integer NOT NULL,
    order_status character varying(100) DEFAULT 'Processing'::character varying NOT NULL,
    paid_at timestamp with time zone,
    delivered_at timestamp with time zone,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.orders OWNER TO jofosuware;

--
-- Name: payments; Type: TABLE; Schema: public; Owner: jofosuware
--

CREATE TABLE public.payments (
    payment_id character varying(300) NOT NULL,
    status character varying(100) NOT NULL,
    order_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.payments OWNER TO jofosuware;

--
-- Name: products; Type: TABLE; Schema: public; Owner: jofosuware
--

CREATE TABLE public.products (
    product_id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name character varying(64) NOT NULL,
    price integer NOT NULL,
    description character varying(1000) NOT NULL,
    ratings integer NOT NULL,
    category character varying(250) NOT NULL,
    seller character varying(250) NOT NULL,
    stock integer NOT NULL,
    num_of_reviews integer DEFAULT 0,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT products_name_check CHECK (((name)::text <> ''::text))
);


ALTER TABLE public.products OWNER TO jofosuware;

--
-- Name: reviews; Type: TABLE; Schema: public; Owner: jofosuware
--

CREATE TABLE public.reviews (
    reviews_id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name character varying(100) NOT NULL,
    ratings integer NOT NULL,
    comment character varying(1000) NOT NULL,
    product_id uuid NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT reviews_name_check CHECK (((name)::text <> ''::text))
);


ALTER TABLE public.reviews OWNER TO jofosuware;

--
-- Name: schema_migration; Type: TABLE; Schema: public; Owner: jofosuware
--

CREATE TABLE public.schema_migration (
    version character varying(14) NOT NULL
);


ALTER TABLE public.schema_migration OWNER TO jofosuware;

--
-- Name: shippings; Type: TABLE; Schema: public; Owner: jofosuware
--

CREATE TABLE public.shippings (
    shipping_id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    address character varying(100) NOT NULL,
    city character varying(100) NOT NULL,
    phone character varying(100) NOT NULL,
    postal character varying(100) NOT NULL,
    country character varying(100) NOT NULL,
    order_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT shippings_address_check CHECK (((address)::text <> ''::text)),
    CONSTRAINT shippings_city_check CHECK (((city)::text <> ''::text)),
    CONSTRAINT shippings_country_check CHECK (((country)::text <> ''::text)),
    CONSTRAINT shippings_phone_check CHECK (((phone)::text <> ''::text)),
    CONSTRAINT shippings_postal_check CHECK (((postal)::text <> ''::text))
);


ALTER TABLE public.shippings OWNER TO jofosuware;

--
-- Name: tokens; Type: TABLE; Schema: public; Owner: jofosuware
--

CREATE TABLE public.tokens (
    token_id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    token_hash bytea NOT NULL,
    expiry timestamp with time zone NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.tokens OWNER TO jofosuware;

--
-- Name: users; Type: TABLE; Schema: public; Owner: jofosuware
--

CREATE TABLE public.users (
    user_id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name character varying(64) NOT NULL,
    email character varying(64) NOT NULL,
    password character varying(250) NOT NULL,
    role character varying(10) DEFAULT 'user'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT users_email_check CHECK (((email)::text <> ''::text)),
    CONSTRAINT users_name_check CHECK (((name)::text <> ''::text)),
    CONSTRAINT users_password_check CHECK ((octet_length((password)::text) <> 0))
);


ALTER TABLE public.users OWNER TO jofosuware;

--
-- Name: avatar avatar_pkey; Type: CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.avatar
    ADD CONSTRAINT avatar_pkey PRIMARY KEY (public_id);


--
-- Name: images images_pkey; Type: CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.images
    ADD CONSTRAINT images_pkey PRIMARY KEY (public_id);


--
-- Name: order_items order_items_pkey; Type: CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_pkey PRIMARY KEY (item_id);


--
-- Name: orders orders_pkey; Type: CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_pkey PRIMARY KEY (order_id);


--
-- Name: payments payments_pkey; Type: CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_pkey PRIMARY KEY (payment_id);


--
-- Name: products products_pkey; Type: CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_pkey PRIMARY KEY (product_id);


--
-- Name: reviews reviews_pkey; Type: CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_pkey PRIMARY KEY (reviews_id);


--
-- Name: schema_migration schema_migration_pkey; Type: CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.schema_migration
    ADD CONSTRAINT schema_migration_pkey PRIMARY KEY (version);


--
-- Name: shippings shippings_pkey; Type: CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.shippings
    ADD CONSTRAINT shippings_pkey PRIMARY KEY (shipping_id);


--
-- Name: tokens tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.tokens
    ADD CONSTRAINT tokens_pkey PRIMARY KEY (token_id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (user_id);


--
-- Name: schema_migration_version_idx; Type: INDEX; Schema: public; Owner: jofosuware
--

CREATE UNIQUE INDEX schema_migration_version_idx ON public.schema_migration USING btree (version);


--
-- Name: avatar avatar_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.avatar
    ADD CONSTRAINT avatar_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: images images_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.images
    ADD CONSTRAINT images_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(product_id) ON DELETE CASCADE;


--
-- Name: order_items order_items_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(order_id) ON DELETE CASCADE;


--
-- Name: order_items order_items_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(product_id) ON DELETE CASCADE;


--
-- Name: orders orders_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: payments payments_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(order_id) ON DELETE CASCADE;


--
-- Name: products products_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: reviews reviews_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(product_id) ON DELETE CASCADE;


--
-- Name: reviews reviews_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: shippings shippings_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.shippings
    ADD CONSTRAINT shippings_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(order_id) ON DELETE CASCADE;


--
-- Name: tokens tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jofosuware
--

ALTER TABLE ONLY public.tokens
    ADD CONSTRAINT tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: FUNCTION uuid_generate_v1(); Type: ACL; Schema: public; Owner: postgres
--

GRANT ALL ON FUNCTION public.uuid_generate_v1() TO jofosuware;


--
-- Name: FUNCTION uuid_generate_v1mc(); Type: ACL; Schema: public; Owner: postgres
--

GRANT ALL ON FUNCTION public.uuid_generate_v1mc() TO jofosuware;


--
-- Name: FUNCTION uuid_generate_v3(namespace uuid, name text); Type: ACL; Schema: public; Owner: postgres
--

GRANT ALL ON FUNCTION public.uuid_generate_v3(namespace uuid, name text) TO jofosuware;


--
-- Name: FUNCTION uuid_generate_v4(); Type: ACL; Schema: public; Owner: postgres
--

GRANT ALL ON FUNCTION public.uuid_generate_v4() TO jofosuware;


--
-- Name: FUNCTION uuid_generate_v5(namespace uuid, name text); Type: ACL; Schema: public; Owner: postgres
--

GRANT ALL ON FUNCTION public.uuid_generate_v5(namespace uuid, name text) TO jofosuware;


--
-- Name: FUNCTION uuid_nil(); Type: ACL; Schema: public; Owner: postgres
--

GRANT ALL ON FUNCTION public.uuid_nil() TO jofosuware;


--
-- Name: FUNCTION uuid_ns_dns(); Type: ACL; Schema: public; Owner: postgres
--

GRANT ALL ON FUNCTION public.uuid_ns_dns() TO jofosuware;


--
-- Name: FUNCTION uuid_ns_oid(); Type: ACL; Schema: public; Owner: postgres
--

GRANT ALL ON FUNCTION public.uuid_ns_oid() TO jofosuware;


--
-- Name: FUNCTION uuid_ns_url(); Type: ACL; Schema: public; Owner: postgres
--

GRANT ALL ON FUNCTION public.uuid_ns_url() TO jofosuware;


--
-- Name: FUNCTION uuid_ns_x500(); Type: ACL; Schema: public; Owner: postgres
--

GRANT ALL ON FUNCTION public.uuid_ns_x500() TO jofosuware;


--
-- Name: DEFAULT PRIVILEGES FOR SEQUENCES; Type: DEFAULT ACL; Schema: -; Owner: postgres
--

ALTER DEFAULT PRIVILEGES FOR ROLE postgres GRANT ALL ON SEQUENCES TO jofosuware;


--
-- Name: DEFAULT PRIVILEGES FOR TYPES; Type: DEFAULT ACL; Schema: -; Owner: postgres
--

ALTER DEFAULT PRIVILEGES FOR ROLE postgres GRANT ALL ON TYPES TO jofosuware;


--
-- Name: DEFAULT PRIVILEGES FOR FUNCTIONS; Type: DEFAULT ACL; Schema: -; Owner: postgres
--

ALTER DEFAULT PRIVILEGES FOR ROLE postgres GRANT ALL ON FUNCTIONS TO jofosuware;


--
-- Name: DEFAULT PRIVILEGES FOR TABLES; Type: DEFAULT ACL; Schema: -; Owner: postgres
--

ALTER DEFAULT PRIVILEGES FOR ROLE postgres GRANT ALL ON TABLES TO jofosuware;


--
-- PostgreSQL database dump complete
--

