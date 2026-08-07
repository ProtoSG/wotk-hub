package games

import "database/sql"

type seedMovie struct {
	emojiStr   string
	answer     string
	difficulty string
}

// Answers are in Spanish (LatAm release titles), matching the app's UI
// language — see games.Guess, which compares the guess against this field
// case-insensitively.
var seedMovies = []seedMovie{
	{"🦇🧛‍♂️🌃", "Batman", "easy"},
	{"🚢🧊💔", "Titanic", "easy"},
	{"🦁👑🌍", "El Rey León", "easy"},
	{"🤠🚀🧸", "Toy Story", "easy"},
	{"❄️👸⛄", "Frozen", "easy"},
	{"⭐⚔️🚀", "Star Wars", "easy"},
	{"🐠🌊🔍", "Buscando a Nemo", "easy"},
	{"🟢🧅👹", "Shrek", "easy"},
	{"🏎️🏁🔧", "Cars", "easy"},
	{"🎈🏠👴", "Up", "easy"},
	{"🦈🌊🚤", "Tiburón", "easy"},
	{"👽🚲🌕", "E.T. el Extraterrestre", "easy"},
	{"🕷️🕸️🧑", "El Hombre Araña", "medium"},
	{"⚡🧙‍♂️🦉", "Harry Potter", "medium"},
	{"🦖🏝️🧬", "Parque Jurásico", "medium"},
	{"💊🕶️🖥️", "Matrix", "medium"},
	{"💙🌳👽", "Avatar", "medium"},
	{"🏴‍☠️⚓🦜", "Piratas del Caribe", "medium"},
	{"🌀🛌🎩", "Origen", "medium"},
	{"🦇🃏🌃", "Batman: El Caballero de la Noche", "medium"},
	{"🦸‍♂️🦸‍♀️🛡️", "Los Vengadores", "medium"},
	{"🐆👑🌍", "Pantera Negra", "medium"},
	{"🚗⚡⏰", "Volver al Futuro", "medium"},
	{"⚔️🛡️🏛️", "Gladiador", "medium"},
	{"🐴🔫🍝", "El Padrino", "hard"},
	{"🌌🚀⏳", "Interstellar", "hard"},
	{"🦸‍♀️🛡️⚡", "Mujer Maravilla", "hard"},
	{"⛓️🕳️📜", "Sueño de Fuga", "hard"},
	{"🥊🧼😵", "El Club de la Pelea", "hard"},
	{"🏃‍♂️🍫🎗️", "Forrest Gump", "hard"},
}

// Seed populates emoji_movies with the starter puzzle set on first run
// only — if the table already has any rows, it's a no-op so replays of
// this call (every backend startup) don't create duplicates.
func Seed(db *sql.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM emoji_movies`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	for _, m := range seedMovies {
		if _, err := db.Exec(
			`INSERT INTO emoji_movies (emoji_str, answer, difficulty) VALUES ($1, $2, $3)`,
			m.emojiStr, m.answer, m.difficulty,
		); err != nil {
			return err
		}
	}
	return nil
}

// ─── Daily Riddle Seed ─────────────────────────────────────────────────────────

type seedRiddle struct {
	question   string
	answer     string
	hint       string
	difficulty string
}

// SeedRiddles populates daily_riddles with the seasonal puzzle set on first
// run only — if the table already has any rows it's a no-op so replays of
// this call (every backend startup) don't create duplicates.
func SeedRiddles(db *sql.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM daily_riddles`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// 90+ riddles in Spanish — classic adivinanzas, logic, wordplay, math,
	// couple-specific, and general knowledge.
	riddles := []seedRiddle{
		// Classic adivinanzas
		{"¿Qué tiene cabeza y no tiene cerebro?", "El ajo", "Piensa en algo que se pela en capas", "easy"},
		{"¿Qué cosa es quella que entre más grande se vuelve, menos pesa?", "El agujero", "Piensa en algo vacío que crece", "easy"},
		{"¿Qué tiene cuello y no tiene cabeza?", "La botella", "Algo que bebes y tiene tapa", "easy"},
		{"¿Qué tiene llaves pero no puede abrir ninguna puerta?", "El piano", "Tiene teclas blancas y negras", "easy"},
		{"¿Qué cosa es quella que todos tienen y nadie la puede perder?", "La sombra", "Te sigue a todas partes", "easy"},
		{"¿Qué tiene dientes pero no puede masticar?", "El peine", "Lo pasas por el cabello", "easy"},
		{"¿Qué entre más quitas, más grande se vuelve?", "El agujero", "Ya salió arriba también", "easy"},
		{"¿Qué cosa corre sin tener patas?", "El agua", "Puedes verla caer de una cascada", "easy"},
		{"¿Qué tiene seis caras y 21 ojos?", "Un dado", "Lo usas en muchos juegos", "easy"},
		{"¿Qué tiene banco pero no tiene dinero?", "El banco del parque", "Te sientas a descansar", "easy"},
		{"¿Qué cosa es roja y azul y si la extiendes te quedas ciego?", "El soldador", "Brilla mucho", "medium"},
		{"¿Qué tiene mariachi y se habla sin cesar?", "El mariachi", "Lleva sombreros grandes", "medium"},
		// Logic puzzles
		{"Un padre y un hijo van en un auto. El padre muere. El hijo es llevado al hospital. El médico dice: 'No puedo operarlo, es mi hijo'. ¿Cómo es posible?", "El médico era su madre", "Piensa en la diversidad de médicos", "medium"},
		{"Hay tres cajas. Una dice 'Premio', otra 'Premio', y la tercera 'Vacío'. Las tres tienen incorrectas las etiquetas. Si solo puedes abrir una caja y sacar un objeto (sin mirar), ¿cómo puedes saber cuál tiene el premio?", "Saca de la caja que dice 'Premio' — si sale vacío, esa es vacía; si sale premio, la otra es vacía y la restante es premio", "Las tres etiquetas están mal, así que la que dice Premio no puede tener Premio", "hard"},
		{"Un hombre vive en el piso 50. Cada día baja al lobby y sube en elevador. Cuando vuelve, solo sube hasta el piso 40 y luego usa las escaleras. ¿Por qué?", "Porque el elevador no tiene botón para el piso 50 desde arriba — es muy alto para llegar con la mano", "El edificio tiene más de 40 pisos", "medium"},
		{"¿Qué número sigue: 1, 11, 21, 1211, 111221, ...?", "312211", "Es una secuencia de descripción", "hard"},
		// Wordplay / doble sentido
		{"¿Por qué el libro de matemáticas estaba triste?", "Porque tenía muchos problemas", "Es un chiste clásico", "easy"},
		{"¿Qué le dijo un techo a otro techo?", "Techo de menos", "Sonido de eco", "easy"},
		{"¿Cómo se dice 'pantalla' en chino?", "No se dice, se habla", "Piensa en la pregunta literal", "easy"},
		{"¿Qué cosa tiene cities pero no houses, forests pero no trees, y agua pero no fish?", "El mapa", "Lo despliegas sobre una mesa", "medium"},
		// Math riddles
		{"¿Cuántas veces puedes restar 5 de 25?", "Una vez — después ya no es 25", "El número cambia", "medium"},
		{"Un granjero tiene 17 ovejas. Todas mueren menos 9. ¿Cuántas quedan?", "9", "Lee bien la pregunta", "easy"},
		{"Si tienes 6 velas encendidas y el viento apaga 2, ¿cuántas velas quedan?", "2 — las otras se derritieron", "Las velas no se van a ningún lado", "medium"},
		{"¿Qué pesa más: un kilo de plumas o un kilo de hierro?", "Pesan igual", "Un kilo es un kilo", "easy"},
		// Couple-specific riddles
		{"¿Dónde fue nuestra primera cita?", "El parque", "Esta respuesta debe ser personalizada", "medium"},
		{"¿Cuál es mi comida favorita?", "Las enchiladas", "Piensa en lo que siempre pides", "medium"},
		{"¿Qué te dije la primera vez que nos vimos?", "Que eras increíble", "Revive ese momento", "hard"},
		{"¿Dónde fue nuestro primer beso?", "Bajo la luna", "Recuerdo de estrellas", "hard"},
		{"¿Cuál fue el primer regalo que te di?", "Una flor", "Fue algo pequeño", "medium"},
		// General knowledge
		{"¿Qué país tiene 4 letras y contiene 'asia'?", "China", "Su capital es Beijing", "easy"},
		{"¿Qué país tiene nombre de color?", "Costa de Marfil", "Traducido del francés", "medium"},
		{"¿Qué planeta es conocido como la estrella de la mañana?", "Venus", "Aparece antes del amanecer", "medium"},
		{"¿Cuántos huesos tiene el cuerpo humano?", "206", "No es un número redondo", "medium"},
		{"¿Qué gas respiramos?", "Oxígeno", "El nitrógeno es el principal del aire", "easy"},
		{"¿Qué océano es el más grande?", "Pacífico", "Cubre más superficie", "easy"},
		{"¿De qué país es Origi?", "Bélgica", "Jugador famoso de fútbol", "medium"},
		{"¿Qué siglo es el año 2025?", "XXI", "Suma los dígitos", "easy"},
		// Riddles - more variety
		{"¿Qué tiene dedos pero no tiene uñas?", "El guante", "Protege tu mano del frío", "easy"},
		{"¿Qué cosa se moja mientras más seca?", "La toalla", "La usas después de bañarte", "easy"},
		{"¿Qué tiene cama pero nunca duerme?", "El río (cama del río)", "Está en la naturaleza", "medium"},
		{"¿Qué cosa tiene boca pero no habla?", "El río (boca del río)", "También tiene corrientes", "medium"},
		{"¿Qué tiene anillos pero no dedos?", "El teléfono", "Y también Saturno", "easy"},
		{"¿Qué tiene ojos pero no ve?", "La aguja", "Tiene un hoyo pequeño", "easy"},
		{"¿Qué tiene patas pero no camina?", "La mesa", "Cuatro patas", "easy"},
		{"¿Qué tiene brazos pero no abraza?", "La silla", "Te sientas en ella", "easy"},
		{"¿Qué tiene espalda pero no puede doler?", "La pared", "Es parte de una habitación", "easy"},
		{"¿Qué tiene frente pero no tiene cara?", "El cuchillo", "Lo usas en la cocina", "easy"},
		{"¿Qué cosa comienza en T, termina en T y solo tiene T?", "El tetris", "Un juego clásico", "easy"},
		{"¿Qué palabra tiene 5 letras pero si le quitas 1 quedan 12?", "Doce — le quitas la 'd' y queda 'once'", "Doce tiene 4 letras, no 5... espera", "hard"},
		{"¿Qué cosa tiene cities, no houses, forests sin trees, water sin fish?", "Un mapa", "Ya salió arriba también", "easy"},
		{"¿Qué tiene 1000 ojos pero no puede ver?", "Un microscopio", "Ayuda a ver lo pequeño", "medium"},
		{"¿Qué tiene 4 letras, 9 sílabas, 1 sonido?", "El silencio", "No se pronuncia", "hard"},
		{"¿Qué coisa que você digita mas não pode enviar?", "O teclado", "Tem letras e números", "medium"},
		{"¿Qué palabra en español tiene 5 letras y contiene 'ua'?", "El agua", "Recurso esencial", "easy"},
		{"¿Qué palabra de 4 letras queda más larga si le agregas 3?", "Larga + 3 = Largaaa", "Piensa en hacer algo más largo", "hard"},
		{"¿Cuánto es 3+3*3-3+3?", "12", "Orden de operaciones", "medium"},
		{"¿Qué palabra de 4 letras puede leer igual al derecho y al revés?", "Radar, nivel, kayak, oteo", "Varias opciones", "medium"},
		{"¿Qué cosa tiene corona pero no es rey?", "El diente", "Está en tu boca", "easy"},
		{"¿Qué tiene sol pero no luz?", "El eclipse", "La bloquea la luna", "medium"},
		{"¿Qué tiene luna pero no es cielo?", "El calendario", "Tiene fases lunares", "medium"},
		{"¿Qué tiene estrellas pero no cielo?", "El cine", "Las ves en la pantalla", "easy"},
		{"¿Qué tiene rayo pero no trueno?", "La fotografía", "Captura luz", "medium"},
		{"¿Qué cosa tiene pico pero no canta?", "El zapato", "Tiene punta", "easy"},
		{"¿Qué tiene ruedas y no es carro?", "La bicicleta", "No tiene motor", "easy"},
		{"¿Qué tiene motor pero no es carro?", "El barco", "También tiene velas", "easy"},
		{"¿Qué tiene timón pero no es barco?", "El avión", "Y también el carro", "easy"},
		{"¿Qué tiene ventanas pero no es casa?", "El computador", "Y también el carro", "easy"},
		{"¿Qué tiene puertas pero no es casa?", "El refrigerador", "Y también el carro", "easy"},
		{"¿Qué tiene techo pero no es casa?", "El carro", "El techo es la parte de arriba", "easy"},
		{"¿Qué tiene cloaca?", "El pato", "Única ave que la tiene", "medium"},
		{"¿Qué animal tiene 4 rodillas?", "El elefante", "Todas sus patas doblan hacia adelante", "hard"},
		{"¿Qué animal es el único que no puede brincar?", "El elefante", "No puede", "medium"},
		{"¿Qué animal tiene bolsa?", "El canguro", "Y también el koala pero menos", "easy"},
		{"¿Qué animal cambia de color?", "El camaleón", "Se confunde con el ambiente", "easy"},
		{"¿Qué animal vive más años?", "La tortuga", "Puede vivir más de 100 años", "easy"},
		{"¿Qué animal tiene 3 corazones?", "El pulpo", "Todos los cefalópodos", "hard"},
		{"¿Qué animal tiene 8 corazones?", "La araña", "Su sangre es diferente", "hard"},
		{"¿Qué animal no tiene sangre?", "El medusa", "Es mayormente agua", "hard"},
		{"¿Qué cosa es más fría que el hielo?", "El nitrógeno líquido", "Y el cero absoluto", "hard"},
		{"¿Qué planeta gira al revés?", "Venus", "Casi todos giran igual", "hard"},
		{"¿Qué planeta tiene día más largo que su año?", "Venus", "Un día dura más que una órbita", "hard"},
		{"¿Qué satélite artificial fue el primero?", "Sputnik 1", "1957", "medium"},
		{"¿Qué evento pasó el 20 de julio de 1969?", "El hombre llegó a la luna", "Neil Armstrong fue primero", "easy"},
		{"¿Qué país creó la pizza?", "Italia", "Nápoles específicamente", "easy"},
		{"¿Qué país inventó el ramen instantáneo?", "Japón", "Momofuku Ando en 1958", "medium"},
		{"¿Qué país tiene forma de bota?", "Italia", "La bota está en el sur de Europa", "easy"},
		{"¿Qué nombre de color es también un nombre de persona?", "Rosa", "Flor y nombre", "easy"},
		{"¿Qué fruta tiene el nombre de un color?", "La naranja", "Color y fruta", "easy"},
		{"¿Qué fruta tiene vitamina C más que la naranja?", "La kiwi", "Y también la fresa", "medium"},
		{"¿Qué cosa es verde por fuera y roja por dentro?", "La sandía", "Roja por dentro", "easy"},
		{"¿Qué cosa tiene semillas por fuera?", "La fresa", "Son las pintitas", "easy"},
		{"¿Qué número es bueno para la suerte?", "7", "Considerado universalmente afortunado", "easy"},
		{"¿Qué número tiene mala suerte en Asia?", "4", "Suena como 'muerte' en chino", "medium"},
		{"¿Qué cosa tiene 7 colores?", "El arcoíris", "Rojo, naranja, amarillo, verde, azul, índigo, violeta", "easy"},
		{"¿Qué cosa tiene 7 notas?", "La escala musical", "Do re mi fa sol la si", "easy"},
		{"¿Qué cosa tiene 7 días?", "La semana", "Cada uno tiene nombre", "easy"},
		{"¿Qué cosa tiene 12 meses?", "El año", "Y también 365 días (o 366)", "easy"},
		{"¿Qué cosa tiene 24 horas?", "El día", "Combinación con la noche = 24h", "easy"},
		{"¿Qué cosa tiene 52 cartas?", "La baraja", "13 de cada palo", "easy"},
		{"¿Qué cosa tiene 0° de latitud?", "El ecuador", "Línea imaginaria", "easy"},
		{"¿Qué cosa tiene forma de esfera?", "La tierra", "Y también una naranja", "easy"},
		{"¿Qué cosa tiene lados?", "El polígono", "Triángulo, cuadrado...", "easy"},
		{"¿Qué cosa tiene vértices?", "El cubo", "8 vértices", "easy"},
		{"¿Qué cosa tiene ángulos?", "El triángulo", "3 ángulos que suman 180°", "easy"},
		{"¿Qué cosa tiene pi?", "El círculo", "Circunferencia / diámetro", "easy"},
		{"¿Qué cosa tiene número áureo?", "La proporción áurea", "Phi = 1.618...", "hard"},
		{"¿Qué palabra es un palíndromo?", "Anita lava la tina", "Se lee igual al revés", "easy"},
		{"¿Qué palabra tiene 4 letras y es una profesión?", "Mago, rey, monja, fraile", "Varias opciones", "easy"},
		{"¿Qué palabra tiene 8 letras y contiene 'te'?", "Contenedor", "Lo usas para guardar cosas", "medium"},
		// Couple-specific riddles continued
		{"¿Cuál es mi apodo cariñoso para ti?", "Mi amor", "Un término de afecto", "easy"},
		{"¿Qué canción suena cuando estamos juntos?", "Nuestra canción", "La que bailamos siempre", "medium"},
		{"¿Dónde nos vimos por primera vez?", "El café", "Fue una tarde", "medium"},
		{"¿Cuál es nuestro color como pareja?", "El rojo", "Color del amor", "easy"},
		{"¿Qué hacemos el domingo?", "Ver películas", "Noche de netflix", "easy"},
	}

	for _, r := range riddles {
		if _, err := db.Exec(
			`INSERT INTO daily_riddles (question, answer, hint, difficulty, published_on)
			 VALUES ($1, $2, $3, $4, '2025-01-01')`,
			r.question, r.answer, r.hint, r.difficulty,
		); err != nil {
			return err
		}
	}
	return nil
}
