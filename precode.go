package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Generator генерирует последовательность чисел 1,2,3 и т.д. и
// отправляет их в канал ch. При этом после записи в канал для каждого числа
// вызывается функция fn. Она служит для подсчёта количества и суммы
// сгенерированных чисел.
func Generator(ctx context.Context, ch chan<- int64, fn func(int64)) {
	defer close(ch)
	var n int64 = 1
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Отправляем сгенерированное число в канал
			ch <- n
			// Вызываем переданную функцию для подсчёта
			fn(n)
			// Генерируем следующее число
			n++
			// Добавим небольшую паузу для наглядности
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Worker читает число из канала in и пишет его в канал out.
func Worker(in <-chan int64, out chan<- int64) {
	for {
		v, ok := <-in
		if !ok {
			// Если канал in закрыт, закрываем канал out и выходим
			close(out)
			return
		}
		// Отправляем значение в канал out
		out <- v
		// Делаем паузу в 1 миллисекунду
		time.Sleep(1 * time.Millisecond)
	}
}

func main() {
	chIn := make(chan int64)

	// 3. Создание контекста
	ctx, cancel := context.WithCancel(context.Background())
	// через 1 секунду вызываем cancel для отмены операции
	time.AfterFunc(1*time.Second, cancel)

	// для проверки будем считать количество и сумму отправленных чисел
	var inputSum int64   // сумма сгенерированных чисел
	var inputCount int64 // количество сгенерированных чисел

	// генерируем числа, считая параллельно их количество и сумму
	go Generator(ctx, chIn, func(i int64) {
		atomic.AddInt64(&inputSum, i)   // Потокобезопасно увеличиваем сумму
		atomic.AddInt64(&inputCount, 1) // Потокобезопасно увеличиваем количество
	})

	const NumOut = 5 // количество обрабатывающих горутин и каналов
	// outs — слайс каналов, куда будут записываться числа из chIn
	outs := make([]chan int64, NumOut)
	for i := 0; i < NumOut; i++ {
		// создаём каналы и для каждого из них вызываем горутину Worker
		outs[i] = make(chan int64)
		go Worker(chIn, outs[i])
	}

	// amounts — слайс, в который собирается статистика по горутинам
	amounts := make([]int64, NumOut)
	// chOut — канал, в который будут отправляться числа из горутин `outs[i]`
	chOut := make(chan int64, NumOut)

	var wg sync.WaitGroup

	// 4. Собираем числа из каналов outs
	for i := 0; i < NumOut; i++ {
		wg.Add(1)
		go func(in <-chan int64, i int) {
			defer wg.Done()
			for v := range in {
				amounts[i]++ // Увеличиваем счётчик обработанных чисел
				chOut <- v   // Передаём значение в chOut
			}
		}(outs[i], i)
	}

	go func() {
		// ждём завершения работы всех горутин для outs
		wg.Wait()
		// закрываем результирующий канал
		close(chOut)
	}()

	var count int64 // количество чисел результирующего канала
	var sum int64   // сумма чисел результирующего канала

	// 5. Читаем числа из результирующего канала
	for v := range chOut {
		count++  // Подсчитываем количество чисел
		sum += v // Суммируем все числа
	}

	finalInputCount := atomic.LoadInt64(&inputCount)
	finalInputSum := atomic.LoadInt64(&inputSum)

	fmt.Println("Количество чисел", finalInputCount, count)
	fmt.Println("Сумма чисел", finalInputSum, sum)
	fmt.Println("Разбивка по каналам", amounts)

	// проверка результатов
	if finalInputSum != sum {
		log.Fatalf("Ошибка: суммы чисел не равны: %d != %d\n", finalInputSum, sum)
	}
	if finalInputCount != count {
		log.Fatalf("Ошибка: количество чисел не равно: %d != %d\n", finalInputCount, count)
	}
	for _, v := range amounts {
		finalInputCount -= v
	}
	if finalInputCount != 0 {
		log.Fatalf("Ошибка: разделение чисел по каналам неверное\n")
	}
}
