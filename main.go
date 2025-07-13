package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

/*
CreateWorkerPool ― запускает amount горутин-воркеров.
Каждый воркер:
  - слушает ctx.Done() – сигнал «остановиться»;
  - берёт число из jobs, возводит его в квадрат и печатает;
  - по выходу вызывает wg.Done(), чтобы main знал, что воркер завершился.
*/
func CreateWorkerPool(ctx context.Context, amount int, jobs <-chan int, wg *sync.WaitGroup) {
	for w := 0; w < amount; w++ {
		wg.Add(1) // учёт горутины в WaitGroup
		go func(id int) {
			defer wg.Done() // гарантированное уменьшение счётчика
			for {
				select {
				// глобальный сигнал остановки
				case <-ctx.Done():
					fmt.Printf("worker %d stopped\n", id)
					return
				// получаем задание
				case num, ok := <-jobs:
					if !ok { // канал закрыт – работы больше нет
						return
					}
					result := num * num
					fmt.Printf("worker %d squared %d → %d\n", id, num, result)
				}
			}
		}(w)
	}
}

func main() {
	// базовый контекст с функцией отмены
	ctx, cancel := context.WithCancel(context.Background())

	// канал для системных сигналов SIGINT/SIGTERM (Ctrl+C)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	// отдельная горутина ждёт сигнал и вызывает cancel()
	go func() {
		<-sig
		cancel() // закрывает ctx.Done() во всей программе
	}()

	jobs := make(chan int, 100) // буфер на 100 задач
	numWorkers := 10            // количество воркеров
	var wg sync.WaitGroup
	CreateWorkerPool(ctx, numWorkers, jobs, &wg)

	// генератор заданий (главная горутина)
	i := 0
	for {
		select {
		case <-ctx.Done(): // получен Ctrl+C
			close(jobs) // больше заданий не будет
			wg.Wait()   // ждём, пока все воркеры выведут «stopped»
			fmt.Println("main stopped")
			return
		default:
			jobs <- i // отправляем очередное число
			i++
			time.Sleep(100 * time.Millisecond) // умеренная скорость генерации
		}
	}
}

//Мною был выбран context для завершения всех-горутин воркеров при получение сигнала прерывания.
// В отличие от канала-флага, контекст ctx.Done() — универсальный канал,который можно передать во все горутин,
// при вызове функцию cancel(), все слушатели получать оповещения. Если бы использовали канал-флаг, то пришлось бы его дублировать в каждый воркер.
//Также использование контекста безопасно, повторонный вызов функции cancel() - не вызовет панику.
//Кроме того, если мы используем контекст, то нам проще масштабироваться,
//если позже мы добавим вторичные операции (HTTP-запросы, БД), им тоже можно передать тот же контекст — они остановятся синхронно.
