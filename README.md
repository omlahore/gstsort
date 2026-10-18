# gstsort

Point it at a folder of GST invoices (PDFs or phone photos) and it gives you a CSV of everything on them, plus a list of the ones you shouldn't claim input tax credit on yet. A local open-weight vision model reads the invoices, so they never leave your machine.

```
$ gstsort -gstin 27AKXPS4821M1Z7 samples/
...
06-typo-gstin.pdf (Lensbox Camera Rentals LB-26-0391)
  - supplier GSTIN "27AAKFL4410C1ZQ" fails the checksum, recheck it on the invoice

07-total-mismatch.pdf (INKWELL STATIONERS IW-2207)
  - taxable 1580.00 + tax 284.40 + cess 0.00 doesn't add up to total 1964.40 (off by -100.00)
  ~ read supplier GSTIN as 27ABZP19054E1Z4, fixed to 27ABZPI9054E1Z4 (checksum confirms)
...
10 invoices, 4 clean. Input tax credit ready to claim: ₹3437.82. Held back for review: ₹2406.56
Wrote invoices.csv
```

## What it checks

The model only reads. Everything after that is plain code:

- GSTIN format and the mod-36 check digit. A misread look-alike (an I read as a 1) gets fixed when the check digit confirms the fix, and a GSTIN that's wrong on the invoice itself gets flagged
- the invoice is billed to your GSTIN, and a bill with no buyer GSTIN is B2C, so there's no credit
- same state means CGST+SGST, different states means IGST
- CGST equals SGST, the tax lands on a real GST slab, and taxable + tax + cess equals the total
- restaurant services (SAC 99633x), usually blocked credit under section 17(5)
- the same supplier and invoice number twice

## Run it

You need [llama.cpp](https://github.com/ggml-org/llama.cpp) and poppler (for PDFs).

```
brew install llama.cpp poppler
llama-server -hf ggml-org/Qwen2.5-VL-7B-Instruct-GGUF:Q4_K_M --port 8091 -c 8192
go install github.com/omlahore/gstsort@latest
gstsort -gstin YOUR_GSTIN path/to/invoices
```

Any OpenAI-compatible server with a vision model works, use `-server` and `-model`. Running it on a hosted API sends your invoices there, which is the thing this tool exists to avoid.

`samples/` has ten made-up invoices (the companies and GSTINs are fake) with a planted problem in most of them.

The checks are a first pass, not tax advice. Your CA still signs off on the return.
