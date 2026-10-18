# Synthetic invoices for testing gstsort. Every company and GSTIN here is made up.
import subprocess, os
C="0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
def chk(s):
    t=0
    for i,ch in enumerate(s):
        p=C.index(ch)*(i%2+1); t+=p//36+p%36
    return C[(36-t%36)%36]
def g(s): return s+chk(s)
BUY=g("27AKXPS4821M1Z"); BUYN="Riya Sharma Design Studio, Baner, Pune"
inv=[
 dict(f="01-nimbus-hosting",t=0,sup="Nimbus Cloud Hosting Pvt Ltd",addr="HSR Layout, Bengaluru, Karnataka",gst=g("29AADCN5512K1Z"),no="NCH/26-27/04417",date="03-10-2026",items=[("Cloud server, 4 vCPU, Sep 2026","998315",1,3400)],mode="igst",rate=18),
 dict(f="02-pixel-cowork",t=1,sup="Pixel Coworking LLP",addr="Kalyani Nagar, Pune, Maharashtra",gst=g("27AAQFP7730H1Z"),no="PX-1093",date="01/10/2026",items=[("Dedicated desk, October 2026","997212",1,8500)],mode="cs",rate=18),
 dict(f="03-swiftnet",t=2,sup="Swiftnet Broadband Services Pvt Ltd",addr="Wakad, Pune, Maharashtra",gst=g("27AAJCS3307B1Z"),no="SWN2610-55821",date="05 Oct 2026",items=[("Fibre 300 Mbps plan, Oct 2026","998422",1,999)],mode="cs",rate=18),
 dict(f="04-kettle-cafe",t=1,sup="Kettle & Co Cafe",addr="Aundh, Pune, Maharashtra",gst=g("27BQRPK6618D1Z"),no="KC/7781",date="07-10-2026",items=[("Client lunch, 3 covers","996331",1,1840)],mode="cs",rate=5),
 dict(f="05-printworks-igst",t=0,sup="Printworks Pune",addr="Shivajinagar, Pune, Maharashtra",gst=g("27AAMFP2209Q1Z"),no="PW/2026/312",date="08-10-2026",items=[("Business cards, 500 pcs","4911",2,650),("Foam board prints","4911",3,420)],mode="igst",rate=18),
 dict(f="06-typo-gstin",t=2,sup="Lensbox Camera Rentals",addr="Andheri West, Mumbai, Maharashtra",gst=None,badgst="27AAKFL4410C1ZQ",no="LB-26-0391",date="10 Oct 2026",items=[("Camera kit rental, 2 days","997329",1,3200)],mode="cs",rate=18),
 dict(f="07-total-mismatch",t=1,sup="Inkwell Stationers",addr="Deccan, Pune, Maharashtra",gst=g("27ABZPI9054E1Z"),no="IW-2207",date="11/10/2026",items=[("Sketchbooks A4","4820",4,240),("Fineliner set","9608",2,310)],mode="cs",rate=18,totaladj=100),
 dict(f="08-nimbus-dup",t=0,sup="Nimbus Cloud Hosting Pvt Ltd",addr="HSR Layout, Bengaluru, Karnataka",gst=g("29AADCN5512K1Z"),no="NCH/26-27/04417",date="03-10-2026",items=[("Cloud server, 4 vCPU, Sep 2026","998315",1,3400)],mode="igst",rate=18),
 dict(f="09-gadget-b2c",t=2,sup="Gadget Galli Electronics",addr="FC Road, Pune, Maharashtra",gst=g("27AFUPG3348L1Z"),no="GG/26/10/0042",date="12 Oct 2026",items=[("USB-C hub 7-in-1","8471",1,2118.64)],mode="cs",rate=18,b2c=True),
 dict(f="10-designfonts-photo",t=0,sup="Typeface Foundry India Pvt Ltd",addr="Indiranagar, Bengaluru, Karnataka",gst=g("29AAGCT8816P1Z"),no="TFI-INV-2026-1188",date="14-10-2026",items=[("Font licence, 2 families, desktop","997331",1,6200)],mode="igst",rate=18,photo=True),
]
CSS=["body{font-family:Helvetica,Arial;margin:40px;color:#111}h1{font-size:26px;margin:0}table{width:100%;border-collapse:collapse;margin-top:18px}td,th{border:1px solid #999;padding:6px;font-size:13px}th{background:#eee}.r{text-align:right}.meta{display:flex;justify-content:space-between;margin-top:14px;font-size:13px}",
"body{font-family:Georgia,serif;margin:50px;color:#222}h1{font-size:22px;letter-spacing:.08em;text-transform:uppercase;border-bottom:3px double #222;padding-bottom:6px}table{width:100%;margin-top:20px;border-collapse:collapse}td,th{padding:5px 4px;font-size:13px;border-bottom:1px dotted #777}.r{text-align:right}.meta{font-size:13px;line-height:1.6}",
"body{font-family:'Courier New',monospace;margin:36px;font-size:13px}h1{font-size:18px}table{width:100%;margin-top:12px}td,th{padding:3px;text-align:left}.r{text-align:right}.meta{margin-top:10px}"]
def html(d):
    gst=d.get("badgst") or d["gst"]
    rows=""; tv=0
    for desc,code,q,rate in d["items"]:
        amt=round(q*rate,2); tv+=amt
        rows+=f"<tr><td>{desc}</td><td>{code}</td><td class=r>{q}</td><td class=r>{rate:,.2f}</td><td class=r>{amt:,.2f}</td></tr>"
    tv=round(tv,2); tax=round(tv*d["rate"]/100,2)
    if d["mode"]=="cs": taxrows=f"<tr><td colspan=4 class=r>CGST @ {d['rate']/2:g}%</td><td class=r>{tax/2:,.2f}</td></tr><tr><td colspan=4 class=r>SGST @ {d['rate']/2:g}%</td><td class=r>{tax/2:,.2f}</td></tr>"
    else: taxrows=f"<tr><td colspan=4 class=r>IGST @ {d['rate']:g}%</td><td class=r>{tax:,.2f}</td></tr>"
    total=round(tv+tax+d.get("totaladj",0),2)
    buyer=f"Bill to: {BUYN}" if d.get("b2c") else f"Bill to: {BUYN}<br>GSTIN: {BUY}<br>Place of supply: 27-Maharashtra"
    return f"""<html><head><style>{CSS[d['t']]}</style></head><body>
<h1>{d['sup']}</h1><div>{d['addr']}<br>GSTIN: {gst}</div>
<div class=meta><div><b>TAX INVOICE</b><br>Invoice No: {d['no']}<br>Date: {d['date']}</div><div>{buyer}</div></div>
<table><tr><th>Description</th><th>HSN/SAC</th><th class=r>Qty</th><th class=r>Rate</th><th class=r>Amount</th></tr>{rows}
<tr><td colspan=4 class=r>Taxable value</td><td class=r>{tv:,.2f}</td></tr>{taxrows}
<tr><td colspan=4 class=r><b>Total</b></td><td class=r><b>₹ {total:,.2f}</b></td></tr></table>
<p style="font-size:11px;margin-top:30px">Sample invoice generated for testing gstsort. Not a real business.</p></body></html>"""
CH="/Applications/Chromium.app/Contents/MacOS/Chromium"
os.makedirs("..",exist_ok=True)
for d in inv:
    h=f"{d['f']}.html"; open(h,"w").write(html(d))
    pdf=os.path.abspath(f"../{d['f']}.pdf")
    subprocess.run([CH,"--headless=new","--disable-gpu",f"--print-to-pdf={pdf}","--no-pdf-header-footer",os.path.abspath(h)],capture_output=True)
    if d.get("photo"):
        subprocess.run(["pdftoppm","-png","-r","200","-singlefile",pdf,"p"],check=True)
        subprocess.run(["magick","p.png","-resize","2600x","-background","#8a7f72","-rotate","4","(","+clone","-sparse-color","Barycentric","0,0 white %w,%h gray62",")","-compose","multiply","-composite","-attenuate","0.35","+noise","Gaussian","-quality","82",f"../{d['f']}.jpg"],check=True)
        os.remove(pdf); os.remove("p.png")
print("buyer",BUY)
