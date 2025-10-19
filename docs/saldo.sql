-- =============================================================================
-- Diagram Name: saldo
-- Created on: 10/19/2025 7:41:58 PM
-- Diagram Version: 
-- =============================================================================

CREATE TABLE "statuses" (
	"statusId" SERIAL NOT NULL,
	"title" varchar(255) NOT NULL,
	"alias" varchar(64) NOT NULL,
	CONSTRAINT "statuses_pkey" PRIMARY KEY("statusId"),
	CONSTRAINT "statuses_alias_key" UNIQUE("alias")
);

CREATE TABLE "users" (
	"userId" int4 NOT NULL,
	"login" varchar(64) NOT NULL,
	"password" varchar(64) NOT NULL,
	"authKey" varchar(32),
	"createdAt" timestamp with time zone NOT NULL DEFAULT now(),
	"lastActivityAt" timestamp with time zone,
	"statusId" int4 NOT NULL,
	CONSTRAINT "users_pkey" PRIMARY KEY("userId")
);

CREATE INDEX "IX_FK_users_statusId_users" ON "users" USING BTREE (
	"statusId"
);


CREATE TABLE "expenses" (
	"expenseId" int4 NOT NULL GENERATED ALWAYS AS IDENTITY,
	"userId" int4 NOT NULL,
	"categoryId" int4,
	"amount" int4 NOT NULL,
	"description" text NOT NULL,
	"createdAt" timestamp with time zone NOT NULL DEFAULT NOW(),
	"updatedAt" timestamp with time zone NOT NULL DEFAULT NOW(),
	"statusId" int4 NOT NULL,
	"currency" varchar(12) NOT NULL,
	PRIMARY KEY("expenseId")
);

CREATE TABLE "categories" (
	"categoryId" int4 NOT NULL GENERATED ALWAYS AS IDENTITY,
	"userId" int4 NOT NULL,
	"title" text NOT NULL,
	"alias" text,
	"createdAt" timestamp with time zone NOT NULL DEFAULT NOW(),
	"updatedAt" timestamp with time zone NOT NULL DEFAULT NOW(),
	"statusId" int4 NOT NULL,
	PRIMARY KEY("categoryId")
);


ALTER TABLE "users" ADD CONSTRAINT "FK_users_statusId" FOREIGN KEY ("statusId")
	REFERENCES "statuses"("statusId")
	MATCH SIMPLE
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "expenses" ADD CONSTRAINT "Ref_expenses_to_statuses" FOREIGN KEY ("statusId")
	REFERENCES "statuses"("statusId")
	MATCH SIMPLE
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "expenses" ADD CONSTRAINT "Ref_expenses_to_users" FOREIGN KEY ("userId")
	REFERENCES "users"("userId")
	MATCH SIMPLE
	ON DELETE NO ACTION
	ON UPDATE NO ACTION
	NOT DEFERRABLE;

ALTER TABLE "expenses" ADD CONSTRAINT "Ref_expenses_to_categories" FOREIGN KEY ("categoryId")
	REFERENCES "categories"("categoryId")
	MATCH SIMPLE
	ON DELETE NO ACTION
	ON UPDATE NO ACTION
	NOT DEFERRABLE;

ALTER TABLE "categories" ADD CONSTRAINT "Ref_categories_to_statuses" FOREIGN KEY ("statusId")
	REFERENCES "statuses"("statusId")
	MATCH SIMPLE
	ON DELETE NO ACTION
	ON UPDATE NO ACTION
	NOT DEFERRABLE;

ALTER TABLE "categories" ADD CONSTRAINT "Ref_categories_to_users" FOREIGN KEY ("userId")
	REFERENCES "users"("userId")
	MATCH SIMPLE
	ON DELETE NO ACTION
	ON UPDATE NO ACTION
	NOT DEFERRABLE;


