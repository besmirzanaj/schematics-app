ALIASES = {
    "alfarome": "Alfa Romeo", "alfa romeo": "Alfa Romeo",
    "aro": "ARO", "austin": "Austin", "autobian": "Autobianchi",
    "bmw": "BMW",
    "daewoo": "Daewoo", "inncenti": "Innocenti",
    "land-rov": "Land Rover", "land rover": "Land Rover",
    "mitsbshi": "Mitsubishi", "mitsubishi": "Mitsubishi",
    "ssang": "SsangYong", "ssangyong": "SsangYong",
    "volkswag": "Volkswagen", "volkswagen": "Volkswagen",
    "mazda": "Mazda", "opel": "Opel", "peugeot": "Peugeot",
    "piaggio": "Piaggio", "porsche": "Porsche", "rover": "Rover",
    "seat": "Seat", "toyota": "Toyota", "jaguar": "Jaguar",
    "ligier": "Ligier",
}

TITLECASE = {
    "audi", "chevrolet", "chrysler", "citroen", "dacia", "daihatsu",
    "dodge", "ferrari", "fiat", "ford", "honda", "hummer", "hyundai", "isuzu",
    "iveco", "jeep", "kia", "lada", "lancia", "lexus", "maruti", "mercedes",
    "mini", "nissan", "renault", "saab", "subaru", "suzuki", "tata", "volvo",
    "bentley", "greatwall", "holden", "infiniti", "lamborghini", "mahindra",
    "scion", "skoda", "smart",
}


def canonical_make(raw: str) -> str:
    low = raw.strip().lower()
    if low in ALIASES:
        return ALIASES[low]
    if low in TITLECASE:
        return low.capitalize()
    return raw.strip()