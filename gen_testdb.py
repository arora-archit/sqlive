"""
gen_testdb.py — generates test.db for sqlive demos.

Schema
──────
  categories   (10 rows)
  products     (120 rows)
  users        (200 rows)
  orders       (500 rows)
  order_items  (1 000 rows)
  reviews      (400 rows)

Showcases
─────────
  SELECT *         — wide tables with many rows
  WHERE  col = val — text / numeric / date filtering
  LIKE             — partial name search
  IN / NOT IN      — status / category sets
  BETWEEN          — price / date ranges
  ORDER BY ASC/DESC
  GROUP BY + COUNT/SUM/AVG/MIN/MAX
  HAVING           — post-aggregation filter
  LIMIT / OFFSET   — pagination
  IS NULL / IS NOT NULL
"""

import sqlite3, random, os
from datetime import date, timedelta

DB = "test.db"
if os.path.exists(DB):
    os.remove(DB)

con = sqlite3.connect(DB)
cur = con.cursor()
cur.execute("PRAGMA journal_mode=WAL")

# ── helpers ───────────────────────────────────────────────────────────────────

def rand_date(start="2020-01-01", end="2025-12-31"):
    s = date.fromisoformat(start)
    e = date.fromisoformat(end)
    return str(s + timedelta(days=random.randint(0, (e - s).days)))

FIRST = ["Alice","Bob","Carol","Dave","Eve","Frank","Grace","Hank","Iris","Jack",
         "Karen","Leo","Mia","Nate","Olivia","Pete","Quinn","Rita","Sam","Tina",
         "Uma","Vic","Wendy","Xander","Yara","Zoe","Aaron","Beth","Carl","Diana"]
LAST  = ["Smith","Jones","Lee","Brown","Taylor","Wilson","Davis","Clark","Hall",
         "Allen","Young","King","Scott","Green","Adams","Baker","Chen","Diaz",
         "Evans","Foster","Gray","Hunt","Irwin","James","Knox","Lane","Moore",
         "Nash","Owen","Perry","Quinn","Reed","Stone","Turner","Upton","Vance"]
CITIES = [
    ("New York","US"), ("Los Angeles","US"), ("Chicago","US"), ("Houston","US"),
    ("Phoenix","US"), ("London","GB"), ("Berlin","DE"), ("Paris","FR"),
    ("Tokyo","JP"), ("Seoul","KR"), ("Sydney","AU"), ("Toronto","CA"),
    ("Madrid","ES"), ("Rome","IT"), ("Amsterdam","NL"), ("Stockholm","SE"),
    ("Warsaw","PL"), ("Lisbon","PT"), ("Vienna","AT"), ("Zurich","CH"),
]
DOMAINS = ["gmail.com","yahoo.com","outlook.com","proton.me","icloud.com","fastmail.com"]
STATUSES = ["active","inactive","banned"]

# ── categories ────────────────────────────────────────────────────────────────

cur.execute("""
CREATE TABLE categories (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT
)""")

categories = [
    (1, "Electronics",   "Gadgets, phones, computers"),
    (2, "Books",         "Fiction, non-fiction, textbooks"),
    (3, "Clothing",      "Apparel for all ages"),
    (4, "Home & Garden", "Furniture and outdoor items"),
    (5, "Sports",        "Equipment and activewear"),
    (6, "Toys",          "Kids and adult games"),
    (7, "Beauty",        "Skincare, makeup, fragrances"),
    (8, "Food",          "Groceries and gourmet items"),
    (9, "Automotive",    "Car parts and accessories"),
    (10,"Travel",        "Luggage and travel gear"),
]
cur.executemany("INSERT INTO categories VALUES (?,?,?)", categories)

# ── products ──────────────────────────────────────────────────────────────────

cur.execute("""
CREATE TABLE products (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    category_id INTEGER REFERENCES categories(id),
    price       REAL NOT NULL,
    stock       INTEGER NOT NULL DEFAULT 0,
    rating      REAL,
    launched_on TEXT,
    discontinued INTEGER NOT NULL DEFAULT 0
)""")

PRODUCT_TEMPLATES = [
    ("Wireless Headphones",1), ("4K Monitor",1), ("Mechanical Keyboard",1),
    ("USB-C Hub",1), ("Smartwatch",1), ("Laptop Stand",1), ("Webcam HD",1),
    ("Noise Cancelling Earbuds",1), ("Portable SSD",1), ("Gaming Mouse",1),
    ("RGB LED Strip",1), ("Smart Plug",1),
    ("Python Crash Course",2), ("Clean Code",2), ("The Pragmatic Programmer",2),
    ("Dune",2), ("Atomic Habits",2), ("Deep Work",2), ("SQL in 10 Minutes",2),
    ("Design Patterns",2), ("The Hobbit",2),
    ("Running Shoes",3), ("Winter Jacket",3), ("Yoga Pants",3), ("Polo Shirt",3),
    ("Hiking Boots",3), ("Compression Socks",3), ("Baseball Cap",3), ("Raincoat",3),
    ("Desk Lamp",4), ("Ergonomic Chair",4), ("Standing Desk",4), ("Plant Pot",4),
    ("Throw Blanket",4), ("Wall Clock",4), ("Picture Frame",4),
    ("Yoga Mat",5), ("Dumbbell Set",5), ("Jump Rope",5), ("Resistance Bands",5),
    ("Foam Roller",5), ("Cycling Gloves",5), ("Water Bottle",5),
    ("LEGO Set",6), ("Puzzle 1000pc",6), ("Card Game",6), ("Board Game",6),
    ("RC Car",6), ("Action Figure",6),
    ("Face Serum",7), ("SPF Moisturiser",7), ("Perfume",7), ("Hair Mask",7),
    ("Lip Balm Set",7), ("Eye Cream",7),
    ("Organic Coffee",8), ("Green Tea",8), ("Dark Chocolate",8), ("Olive Oil",8),
    ("Hot Sauce",8), ("Protein Powder",8),
    ("Car Phone Mount",9), ("Dash Cam",9), ("Tire Inflator",9), ("Car Vacuum",9),
    ("OBD Scanner",9),
    ("Travel Pillow",10), ("Packing Cubes",10), ("Luggage Lock",10),
    ("Passport Holder",10), ("Portable Charger",10), ("Travel Adapter",10),
]

products = []
for idx, (name, cat) in enumerate(PRODUCT_TEMPLATES, 1):
    base = round(random.uniform(4.99, 599.99), 2)
    stock = random.randint(0, 500)
    rating = round(random.uniform(2.5, 5.0), 1) if random.random() > 0.05 else None
    launched = rand_date("2018-01-01", "2024-06-01")
    disc = 1 if random.random() < 0.08 else 0
    products.append((idx, name, cat, base, stock, rating, launched, disc))

# Add some variants to bulk it up to 120
for extra in range(len(PRODUCT_TEMPLATES)+1, 121):
    tmpl_name, cat = random.choice(PRODUCT_TEMPLATES)
    name = f"{tmpl_name} v{random.randint(2,5)}"
    base = round(random.uniform(4.99, 599.99), 2)
    stock = random.randint(0, 500)
    rating = round(random.uniform(2.5, 5.0), 1) if random.random() > 0.05 else None
    launched = rand_date("2018-01-01", "2024-06-01")
    disc = 1 if random.random() < 0.1 else 0
    products.append((extra, name, cat, base, stock, rating, launched, disc))

cur.executemany("INSERT INTO products VALUES (?,?,?,?,?,?,?,?)", products)

# ── users ─────────────────────────────────────────────────────────────────────

cur.execute("""
CREATE TABLE users (
    id          INTEGER PRIMARY KEY,
    first_name  TEXT NOT NULL,
    last_name   TEXT NOT NULL,
    email       TEXT NOT NULL UNIQUE,
    age         INTEGER,
    city        TEXT,
    country     TEXT,
    status      TEXT NOT NULL DEFAULT 'active',
    joined_on   TEXT NOT NULL,
    last_login  TEXT
)""")

users = []
seen_emails = set()
for uid in range(1, 201):
    fn = random.choice(FIRST)
    ln = random.choice(LAST)
    domain = random.choice(DOMAINS)
    email = f"{fn.lower()}.{ln.lower()}{uid}@{domain}"
    while email in seen_emails:
        email = f"{fn.lower()}{random.randint(1,999)}@{domain}"
    seen_emails.add(email)
    age = random.randint(18, 72) if random.random() > 0.03 else None
    city, country = random.choice(CITIES)
    status = random.choices(STATUSES, weights=[85, 12, 3])[0]
    joined = rand_date("2019-01-01", "2024-12-01")
    last_login = rand_date(joined, "2025-12-31") if random.random() > 0.08 else None
    users.append((uid, fn, ln, email, age, city, country, status, joined, last_login))

cur.executemany("INSERT INTO users VALUES (?,?,?,?,?,?,?,?,?,?)", users)

# ── orders ────────────────────────────────────────────────────────────────────

cur.execute("""
CREATE TABLE orders (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id),
    status       TEXT NOT NULL DEFAULT 'pending',
    total_amount REAL NOT NULL,
    discount     REAL NOT NULL DEFAULT 0,
    placed_on    TEXT NOT NULL,
    shipped_on   TEXT,
    notes        TEXT
)""")

ORDER_STATUSES = ["pending","processing","shipped","delivered","cancelled","refunded"]
ORDER_NOTES = ["Leave at door","Ring bell","Fragile","Gift wrap","No rush", None, None, None]

orders = []
for oid in range(1, 501):
    uid = random.randint(1, 200)
    status = random.choices(ORDER_STATUSES, weights=[5,10,15,55,10,5])[0]
    total = round(random.uniform(5.0, 950.0), 2)
    discount = round(random.uniform(0, min(total * 0.3, 100)), 2) if random.random() > 0.6 else 0.0
    placed = rand_date("2022-01-01", "2025-12-01")
    shipped = rand_date(placed, "2025-12-15") if status not in ("pending","processing","cancelled") else None
    note = random.choice(ORDER_NOTES)
    orders.append((oid, uid, status, total, discount, placed, shipped, note))

cur.executemany("INSERT INTO orders VALUES (?,?,?,?,?,?,?,?)", orders)

# ── order_items ───────────────────────────────────────────────────────────────

cur.execute("""
CREATE TABLE order_items (
    id         INTEGER PRIMARY KEY,
    order_id   INTEGER NOT NULL REFERENCES orders(id),
    product_id INTEGER NOT NULL REFERENCES products(id),
    quantity   INTEGER NOT NULL DEFAULT 1,
    unit_price REAL NOT NULL
)""")

items = []
iid = 1
for oid in range(1, 501):
    n_items = random.randint(1, 4)
    picked = random.sample(range(1, len(products)+1), min(n_items, len(products)))
    for pid in picked:
        # look up product price
        price = next(p[3] for p in products if p[0] == pid)
        qty = random.randint(1, 5)
        items.append((iid, oid, pid, qty, price))
        iid += 1
        if iid > 1000:
            break
    if iid > 1000:
        break

cur.executemany("INSERT INTO order_items VALUES (?,?,?,?,?)", items)

# ── reviews ───────────────────────────────────────────────────────────────────

cur.execute("""
CREATE TABLE reviews (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id),
    product_id INTEGER NOT NULL REFERENCES products(id),
    rating     INTEGER NOT NULL CHECK(rating BETWEEN 1 AND 5),
    title      TEXT,
    body       TEXT,
    helpful    INTEGER NOT NULL DEFAULT 0,
    created_on TEXT NOT NULL
)""")

TITLES = [
    "Great product!", "Disappointed", "Worth every penny", "Not as described",
    "Highly recommend", "Average at best", "Exceeded expectations", "Would buy again",
    "Broke after a week", "Best purchase this year", "Solid build quality",
    "Overpriced", "Perfect gift", "Good value for money", None, None,
]
BODIES = [
    "Really happy with this purchase.", "Arrived damaged unfortunately.",
    "Exactly as described, fast shipping.", "Quality is not great for the price.",
    "My family loves it.", "Could be better.", "Will definitely buy again.",
    "Customer support was very helpful.", None, None, None,
]

reviews = []
for rid in range(1, 401):
    uid = random.randint(1, 200)
    pid = random.randint(1, len(products))
    rating = random.choices([1,2,3,4,5], weights=[5,8,15,32,40])[0]
    title = random.choice(TITLES)
    body = random.choice(BODIES)
    helpful = random.randint(0, 150)
    created = rand_date("2020-06-01", "2025-12-01")
    reviews.append((rid, uid, pid, rating, title, body, helpful, created))

cur.executemany("INSERT INTO reviews VALUES (?,?,?,?,?,?,?,?)", reviews)

con.commit()
con.close()

print(f"✓  {DB} created")
print("   categories  : 10 rows")
print(f"   products    : {len(products)} rows")
print(f"   users       : {len(users)} rows")
print(f"   orders      : {len(orders)} rows")
print(f"   order_items : {len(items)} rows")
print(f"   reviews     : {len(reviews)} rows")
print()
print("Sample queries to try:")
print("  SELECT * FROM users WHERE country = 'US' ORDER BY joined_on DESC LIMIT 25")
print("  SELECT category_id, COUNT(*), AVG(price), MIN(price), MAX(price) FROM products GROUP BY category_id HAVING AVG(price) > 50")
print("  SELECT status, COUNT(*), SUM(total_amount) FROM orders GROUP BY status ORDER BY COUNT(*) DESC")
print("  SELECT first_name, last_name, city FROM users WHERE status = 'active' AND age BETWEEN 25 AND 40")
print("  SELECT name, price FROM products WHERE price > 100 AND discontinued = 0 ORDER BY rating DESC LIMIT 10")
print("  SELECT * FROM reviews WHERE rating = 5 AND helpful > 50 ORDER BY created_on DESC")
